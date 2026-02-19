package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aigustalabs/switchboard/bridges/protocol"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "google.golang.org/protobuf/proto"
)

// handleChatsList retrieves known contacts/groups and emits a chats.list response.
func handleChatsList(client *whatsmeow.Client, writer *protocol.Writer, reqID string) {
	if !client.IsConnected() {
		fmt.Fprintln(os.Stderr, "chats.list: not connected")
		if err := writer.SendTyped("chats.list", reqID, protocol.ChatListResponse{
			Chats: []protocol.Chat{},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "send empty chats.list: %v\n", err)
		}
		return
	}

	// GetAllContacts returns a map[types.JID]types.ContactInfo.
	contacts, err := client.Store.Contacts.GetAllContacts(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "get contacts: %v\n", err)
		contacts = map[types.JID]types.ContactInfo{}
	}

	chats := make([]protocol.Chat, 0, len(contacts))
	for jid, info := range contacts {
		name := info.FullName
		if name == "" {
			name = info.PushName
		}
		if name == "" {
			name = jid.User
		}
		isGroup := jid.Server == types.GroupServer
		chats = append(chats, protocol.Chat{
			ID:      jid.String(),
			Name:    name,
			IsGroup: isGroup,
		})
	}

	if err := writer.SendTyped("chats.list", reqID, protocol.ChatListResponse{
		Chats: chats,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "send chats.list: %v\n", err)
	}
}

// handleChatMessages returns an empty message list.
// whatsmeow does not provide server-side message history for new sessions.
func handleChatMessages(client *whatsmeow.Client, writer *protocol.Writer, reqID string, req protocol.ChatMessagesRequest) {
	if err := writer.SendTyped("chat.messages", reqID, protocol.ChatMessagesResponse{
		Messages: []protocol.Message{},
	}); err != nil {
		fmt.Fprintf(os.Stderr, "send chat.messages: %v\n", err)
	}
}

// handleSendMessage sends a text message to the specified chat.
func handleSendMessage(client *whatsmeow.Client, writer *protocol.Writer, reqID string, req protocol.SendMessageRequest) {
	if !client.IsConnected() {
		fmt.Fprintln(os.Stderr, "message.send: not connected")
		return
	}

	jid, err := types.ParseJID(req.ChatID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse JID %q: %v\n", req.ChatID, err)
		return
	}

	msg := &waE2E.Message{
		Conversation: waProto.String(req.Text),
	}

	resp, err := client.SendMessage(context.Background(), jid, msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "send message to %s: %v\n", req.ChatID, err)
		return
	}

	// Emit the sent message back as message.new so the UI can display it.
	out := protocol.Message{
		ID:        resp.ID,
		ChatID:    req.ChatID,
		From:      "me",
		FromMe:    true,
		Text:      req.Text,
		Timestamp: resp.Timestamp.Unix(),
	}
	if err := writer.SendTyped("message.new", reqID, out); err != nil {
		fmt.Fprintf(os.Stderr, "send message.new for sent msg: %v\n", err)
	}
}

// handleEvent processes incoming whatsmeow events and emits protocol messages.
func handleEvent(client *whatsmeow.Client, writer *protocol.Writer, mediaDir string, rawEvt interface{}) {
	switch evt := rawEvt.(type) {
	case *events.Connected:
		handleConnectedEvent(client, writer, evt)

	case *events.LoggedOut:
		fmt.Fprintln(os.Stderr, "logged out")
		if err := writer.SendTyped("status", "", protocol.StatusData{Status: "auth_needed"}); err != nil {
			fmt.Fprintf(os.Stderr, "send status auth_needed: %v\n", err)
		}

	case *events.Disconnected:
		fmt.Fprintln(os.Stderr, "disconnected")
		if err := writer.SendTyped("status", "", protocol.StatusData{Status: "disconnected"}); err != nil {
			fmt.Fprintf(os.Stderr, "send status disconnected: %v\n", err)
		}

	case *events.Message:
		handleIncomingMessage(client, writer, mediaDir, evt)
	}
}

// handleIncomingMessage processes a received message event.
func handleIncomingMessage(client *whatsmeow.Client, writer *protocol.Writer, mediaDir string, evt *events.Message) {
	info := evt.Info
	chatID := info.Chat.String()
	senderID := info.Sender.String()
	msgID := string(info.ID)
	ts := info.Timestamp.Unix()

	text := ""
	imagePath := ""

	msg := evt.Message
	if msg == nil {
		return
	}

	// Extract text content.
	if msg.GetConversation() != "" {
		text = msg.GetConversation()
	} else if msg.GetExtendedTextMessage() != nil {
		text = msg.GetExtendedTextMessage().GetText()
	}

	// Download image if present.
	if imgMsg := msg.GetImageMessage(); imgMsg != nil {
		if text == "" {
			text = imgMsg.GetCaption()
		}
		data, err := client.Download(context.Background(), imgMsg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "download image in msg %s: %v\n", msgID, err)
		} else {
			// Save with a hash-based filename.
			hash := sha256.Sum256(data)
			fname := fmt.Sprintf("%x.jpg", hash[:8])
			fpath := filepath.Join(mediaDir, fname)
			if err := os.WriteFile(fpath, data, 0o600); err != nil {
				fmt.Fprintf(os.Stderr, "write image %s: %v\n", fpath, err)
			} else {
				imagePath = fpath
			}
		}
	}

	out := protocol.Message{
		ID:        msgID,
		ChatID:    chatID,
		From:      senderID,
		FromMe:    info.IsFromMe,
		Text:      text,
		Timestamp: ts,
		ImagePath: imagePath,
	}

	if err := writer.SendTyped("message.new", "", out); err != nil {
		fmt.Fprintf(os.Stderr, "send message.new: %v\n", err)
	}

	// Only notify for messages from others.
	if !info.IsFromMe && (text != "" || imagePath != "") {
		// Resolve a display name for the sender.
		senderName := info.PushName
		if senderName == "" {
			senderName = info.Sender.User
		}
		body := text
		if body == "" && imagePath != "" {
			body = "[image]"
		}
		notif := protocol.Notification{
			Title:   senderName,
			Body:    body,
			Service: "whatsapp",
		}
		if err := writer.SendTyped("notification", "", notif); err != nil {
			fmt.Fprintf(os.Stderr, "send notification: %v\n", err)
		}
	}
}
