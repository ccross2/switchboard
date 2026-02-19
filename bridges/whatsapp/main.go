package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/aigustalabs/switchboard/bridges/protocol"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func main() {
	// All logging goes to stderr — stdout is IPC.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Ensure config directories exist.
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "switchboard")
	dbPath := filepath.Join(configDir, "whatsapp.db")
	mediaDir := filepath.Join(configDir, "media", "whatsapp")

	if err := os.MkdirAll(configDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "create config dir: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(mediaDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "create media dir: %v\n", err)
		os.Exit(1)
	}

	// Set up protocol writer/reader.
	writer := protocol.NewWriter()
	reader := protocol.NewReader()

	// Open SQLite store.
	dbLogger := waLog.Stdout("DB", "WARN", true)
	container, err := sqlstore.New(context.Background(), "sqlite3",
		fmt.Sprintf("file:%s?_foreign_keys=on", dbPath), dbLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open store: %v\n", err)
		os.Exit(1)
	}

	device, err := container.GetFirstDevice(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "get device: %v\n", err)
		os.Exit(1)
	}

	clientLogger := waLog.Stdout("Client", "WARN", true)
	client := whatsmeow.NewClient(device, clientLogger)

	// Register event handler.
	client.AddEventHandler(func(evt interface{}) {
		handleEvent(client, writer, mediaDir, evt)
	})

	// Start stdin command loop in background.
	go runCommandLoop(reader, writer, client, mediaDir)

	// Connect to WhatsApp.
	if client.Store.ID == nil {
		// No session — need to authenticate.
		if err := writer.SendTyped("status", "", protocol.StatusData{Status: "auth_needed"}); err != nil {
			fmt.Fprintf(os.Stderr, "send status: %v\n", err)
		}
	} else {
		if err := client.Connect(); err != nil {
			fmt.Fprintf(os.Stderr, "connect: %v\n", err)
			os.Exit(1)
		}
	}

	// Wait for termination.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Fprintln(os.Stderr, "shutting down")
	client.Disconnect()
}

// runCommandLoop reads JSON-line commands from stdin and dispatches them.
func runCommandLoop(reader *protocol.Reader, writer *protocol.Writer, client *whatsmeow.Client, mediaDir string) {
	for {
		env, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				fmt.Fprintln(os.Stderr, "stdin closed")
				return
			}
			fmt.Fprintf(os.Stderr, "read command: %v\n", err)
			continue
		}

		switch env.Type {
		case "auth.start":
			go handleQRLogin(client, writer)

		case "chats.list":
			go handleChatsList(client, writer, env.ID)

		case "chat.messages":
			var req protocol.ChatMessagesRequest
			if err := protocol.ParseData(env, &req); err != nil {
				fmt.Fprintf(os.Stderr, "parse chat.messages: %v\n", err)
				continue
			}
			go handleChatMessages(client, writer, env.ID, req)

		case "message.send":
			var req protocol.SendMessageRequest
			if err := protocol.ParseData(env, &req); err != nil {
				fmt.Fprintf(os.Stderr, "parse message.send: %v\n", err)
				continue
			}
			go handleSendMessage(client, writer, env.ID, req)

		default:
			fmt.Fprintf(os.Stderr, "unknown command type: %s\n", env.Type)
		}
	}
}
