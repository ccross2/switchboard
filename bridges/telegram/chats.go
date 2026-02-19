package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aigustalabs/switchboard/bridges/protocol"
	"github.com/gotd/td/tg"
)

// handleChatsList fetches the user's dialogs and emits a chats.list response.
func handleChatsList(ctx context.Context, client *tgClient, writer *protocol.Writer, id string) {
	api := client.tg.API()

	result, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      100,
	})
	if err != nil {
		log.Printf("[chats] GetDialogs error: %v\n", err)
		_ = writer.SendTyped("error", id, map[string]string{"message": err.Error()})
		return
	}

	chats := extractChats(result)
	_ = writer.SendTyped("chats.list", id, protocol.ChatListResponse{Chats: chats})
}

// extractChats converts a dialogs result into the protocol Chat slice.
func extractChats(result tg.MessagesDialogsClass) []protocol.Chat {
	var (
		rawDialogs []tg.DialogClass
		usersList  []tg.UserClass
		rawChats   []tg.ChatClass
		messages   []tg.MessageClass
	)

	switch d := result.(type) {
	case *tg.MessagesDialogs:
		rawDialogs = d.Dialogs
		usersList = d.Users
		rawChats = d.Chats
		messages = d.Messages
	case *tg.MessagesDialogsSlice:
		rawDialogs = d.Dialogs
		usersList = d.Users
		rawChats = d.Chats
		messages = d.Messages
	default:
		return nil
	}

	// Build lookup maps from interface slices via type assertions.
	userMap := make(map[int64]*tg.User, len(usersList))
	for _, u := range usersList {
		if cu, ok := u.(*tg.User); ok {
			userMap[cu.ID] = cu
		}
	}

	chatMap := make(map[int64]tg.ChatClass, len(rawChats))
	for _, c := range rawChats {
		switch ch := c.(type) {
		case *tg.Chat:
			chatMap[ch.ID] = ch
		case *tg.Channel:
			chatMap[ch.ID] = ch
		}
	}

	// Index last messages by their ID.
	msgMap := make(map[int]tg.MessageClass, len(messages))
	for _, m := range messages {
		if msg, ok := m.(*tg.Message); ok {
			msgMap[msg.ID] = m
		}
	}

	var out []protocol.Chat
	for _, dlgRaw := range rawDialogs {
		d, ok := dlgRaw.(*tg.Dialog)
		if !ok {
			continue
		}

		var (
			chatID   string
			chatName string
			isGroup  bool
		)

		switch p := d.Peer.(type) {
		case *tg.PeerUser:
			chatID = strconv.FormatInt(p.UserID, 10)
			if u, ok := userMap[p.UserID]; ok {
				chatName = u.FirstName
				if u.LastName != "" {
					chatName += " " + u.LastName
				}
				if chatName == "" {
					chatName = u.Username
				}
			}
		case *tg.PeerChat:
			chatID = "c_" + strconv.FormatInt(p.ChatID, 10)
			isGroup = true
			if c, ok := chatMap[p.ChatID]; ok {
				if ch, ok := c.(*tg.Chat); ok {
					chatName = ch.Title
				}
			}
		case *tg.PeerChannel:
			chatID = "ch_" + strconv.FormatInt(p.ChannelID, 10)
			if c, ok := chatMap[p.ChannelID]; ok {
				if ch, ok := c.(*tg.Channel); ok {
					chatName = ch.Title
					isGroup = ch.Megagroup || ch.Broadcast
				}
			}
		default:
			continue
		}

		if chatName == "" {
			chatName = chatID
		}

		var lastMsg string
		var lastTime int64
		if topMsg, ok := msgMap[d.TopMessage]; ok {
			if m, ok := topMsg.(*tg.Message); ok {
				lastMsg = m.Message
				lastTime = int64(m.Date)
			}
		}

		out = append(out, protocol.Chat{
			ID:          chatID,
			Name:        chatName,
			UnreadCount: d.UnreadCount,
			LastMessage: lastMsg,
			LastTime:    lastTime,
			IsGroup:     isGroup,
		})
	}
	return out
}

// handleChatMessages fetches history for a chat and emits a chat.messages response.
func handleChatMessages(ctx context.Context, client *tgClient, writer *protocol.Writer, id string, req protocol.ChatMessagesRequest) {
	peer, err := parsePeer(req.ChatID)
	if err != nil {
		_ = writer.SendTyped("error", id, map[string]string{"message": err.Error()})
		return
	}

	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	api := client.tg.API()
	result, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: limit,
	})
	if err != nil {
		log.Printf("[chats] GetHistory error: %v\n", err)
		_ = writer.SendTyped("error", id, map[string]string{"message": err.Error()})
		return
	}

	messages := extractMessages(result, req.ChatID, client.mediaDir)
	_ = writer.SendTyped("chat.messages", id, protocol.ChatMessagesResponse{Messages: messages})
}

// extractMessages converts a history result into the protocol Message slice.
func extractMessages(result tg.MessagesMessagesClass, chatID string, mediaDir string) []protocol.Message {
	var rawMsgs []tg.MessageClass
	var usersList []tg.UserClass

	switch r := result.(type) {
	case *tg.MessagesMessages:
		rawMsgs = r.Messages
		usersList = r.Users
	case *tg.MessagesMessagesSlice:
		rawMsgs = r.Messages
		usersList = r.Users
	case *tg.MessagesChannelMessages:
		rawMsgs = r.Messages
		usersList = r.Users
	default:
		return nil
	}

	userMap := make(map[int64]*tg.User, len(usersList))
	for _, u := range usersList {
		if cu, ok := u.(*tg.User); ok {
			userMap[cu.ID] = cu
		}
	}

	var out []protocol.Message
	for _, raw := range rawMsgs {
		msg, ok := raw.(*tg.Message)
		if !ok {
			continue
		}

		fromName := "Unknown"
		fromMe := msg.Out
		if msg.FromID != nil {
			if pu, ok := msg.FromID.(*tg.PeerUser); ok {
				if u, ok := userMap[pu.UserID]; ok {
					fromName = u.FirstName
					if u.LastName != "" {
						fromName += " " + u.LastName
					}
					if fromName == "" {
						fromName = u.Username
					}
				}
			}
		}

		var imagePath string
		if msg.Media != nil {
			if photo, ok := msg.Media.(*tg.MessageMediaPhoto); ok {
				imagePath = downloadPhoto(photo, mediaDir)
			}
		}

		out = append(out, protocol.Message{
			ID:        strconv.Itoa(msg.ID),
			ChatID:    chatID,
			From:      fromName,
			FromMe:    fromMe,
			Text:      msg.Message,
			Timestamp: int64(msg.Date),
			ImagePath: imagePath,
		})
	}
	return out
}

// handleSendMessage sends a text message to a chat and emits an ack.
func handleSendMessage(ctx context.Context, client *tgClient, writer *protocol.Writer, id string, req protocol.SendMessageRequest) {
	peer, err := parsePeer(req.ChatID)
	if err != nil {
		_ = writer.SendTyped("error", id, map[string]string{"message": err.Error()})
		return
	}

	api := client.tg.API()
	randomID := rand.Int63() //nolint:gosec — not security-sensitive
	_, err = api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:      peer,
		Message:   req.Text,
		RandomID:  randomID,
		NoWebpage: true,
	})
	if err != nil {
		log.Printf("[chats] SendMessage error: %v\n", err)
		_ = writer.SendTyped("error", id, map[string]string{"message": err.Error()})
		return
	}

	_ = writer.SendTyped("message.sent", id, map[string]string{"chat_id": req.ChatID})
}

// parsePeer converts a Switchboard chat ID string to a tg.InputPeerClass.
// Conventions:
//   - plain integer → InputPeerUser
//   - "c_<int>"    → InputPeerChat (basic group)
//   - "ch_<int>"   → InputPeerChannel (supergroup/channel)
func parsePeer(chatID string) (tg.InputPeerClass, error) {
	if len(chatID) > 3 && chatID[:3] == "ch_" {
		n, err := strconv.ParseInt(chatID[3:], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid channel id %q", chatID)
		}
		return &tg.InputPeerChannel{ChannelID: n}, nil
	}
	if len(chatID) > 2 && chatID[:2] == "c_" {
		n, err := strconv.ParseInt(chatID[2:], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid chat id %q", chatID)
		}
		return &tg.InputPeerChat{ChatID: n}, nil
	}
	n, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user id %q", chatID)
	}
	return &tg.InputPeerUser{UserID: n}, nil
}

// downloadPhoto saves the largest photo size to the media dir and returns the path.
// Returns an empty string if downloading fails (non-fatal).
func downloadPhoto(photo *tg.MessageMediaPhoto, mediaDir string) string {
	p, ok := photo.Photo.(*tg.Photo)
	if !ok {
		return ""
	}

	// Find the largest PhotoSize.
	var bestType string
	var bestW int
	for _, sz := range p.Sizes {
		if s, ok := sz.(*tg.PhotoSize); ok {
			if s.W > bestW {
				bestW = s.W
				bestType = s.Type
			}
		}
	}
	if bestType == "" {
		return ""
	}

	localPath := filepath.Join(mediaDir, fmt.Sprintf("photo_%d_%s.jpg", p.ID, bestType))
	if _, err := os.Stat(localPath); err == nil {
		return localPath // already cached
	}

	// Attempt an HTTP download using the DC-independent photo URL format.
	// This is a best-effort attempt; failures are non-fatal.
	dlURL := fmt.Sprintf("https://t.me/i/userpic/%d/%d", p.ID, p.AccessHash)
	//nolint:gosec — user-visible media URL
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Get(dlURL)
	if err != nil {
		log.Printf("[media] download error for %s: %v\n", dlURL, err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[media] download status %d for %s\n", resp.StatusCode, dlURL)
		return ""
	}

	f, err := os.Create(localPath)
	if err != nil {
		log.Printf("[media] create file %s: %v\n", localPath, err)
		return ""
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		log.Printf("[media] write file %s: %v\n", localPath, err)
		os.Remove(localPath)
		return ""
	}

	return localPath
}

// buildIncomingMessage converts a tg.Message from an update into a protocol.Message.
func buildIncomingMessage(msg *tg.Message, chatID string) protocol.Message {
	return protocol.Message{
		ID:        strconv.Itoa(msg.ID),
		ChatID:    chatID,
		From:      "unknown",
		FromMe:    msg.Out,
		Text:      msg.Message,
		Timestamp: int64(msg.Date),
	}
}

// peerToChatID converts a tg.PeerClass to a Switchboard chat ID string.
func peerToChatID(peer tg.PeerClass) string {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return strconv.FormatInt(p.UserID, 10)
	case *tg.PeerChat:
		return "c_" + strconv.FormatInt(p.ChatID, 10)
	case *tg.PeerChannel:
		return "ch_" + strconv.FormatInt(p.ChannelID, 10)
	}
	return "unknown"
}
