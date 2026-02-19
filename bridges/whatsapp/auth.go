package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aigustalabs/switchboard/bridges/protocol"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// handleQRLogin initiates QR-code pairing for a client with no stored session.
// It emits auth.qr events for each QR code, and auth.success on completion.
func handleQRLogin(client *whatsmeow.Client, writer *protocol.Writer) {
	if client.IsConnected() {
		client.Disconnect()
	}

	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "get QR channel: %v\n", err)
		return
	}

	if err := client.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "connect for QR login: %v\n", err)
		return
	}

	for item := range qrChan {
		switch item.Event {
		case "code":
			if err := writer.SendTyped("auth.qr", "", protocol.AuthQR{Code: item.Code}); err != nil {
				fmt.Fprintf(os.Stderr, "send auth.qr: %v\n", err)
			}
		case "success":
			jid := client.Store.ID
			user := ""
			phone := ""
			if jid != nil {
				user = jid.User
				phone = jid.User // WhatsApp JID User field is the phone number
			}
			if err := writer.SendTyped("auth.success", "", protocol.AuthSuccess{
				User:  user,
				Phone: phone,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "send auth.success: %v\n", err)
			}
		case "error":
			fmt.Fprintf(os.Stderr, "QR channel error: %v\n", item.Error)
		case "timeout":
			fmt.Fprintln(os.Stderr, "QR code timed out")
		default:
			fmt.Fprintf(os.Stderr, "QR channel event: %s\n", item.Event)
		}
	}
}

// handleConnectedEvent handles a successful connection/reconnection event.
// Called from the event handler when events.Connected is received.
func handleConnectedEvent(client *whatsmeow.Client, writer *protocol.Writer, evt *events.Connected) {
	jid := client.Store.ID
	user := ""
	phone := ""
	if jid != nil {
		user = jid.User
		phone = jid.User
	}
	if err := writer.SendTyped("auth.success", "", protocol.AuthSuccess{
		User:  user,
		Phone: phone,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "send auth.success on reconnect: %v\n", err)
	}
}
