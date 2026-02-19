package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/aigustalabs/switchboard/bridges/protocol"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
)

// tgClient wraps a connected gotd telegram.Client together with shared state.
type tgClient struct {
	tg       *telegram.Client
	writer   *protocol.Writer
	mediaDir string
	// af is the active auth flow (nil when not in progress).
	af *authFlow
}

func main() {
	// Route all log output to stderr so stdout stays clean for IPC.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	// --- Credentials ---
	apiID, apiHash, err := loadCredentials()
	if err != nil {
		log.Fatalf("[main] credentials: %v\n", err)
	}

	// --- Config dirs ---
	configDir := filepath.Join(os.Getenv("HOME"), ".config", "switchboard")
	mediaDir := filepath.Join(configDir, "media", "telegram")
	sessionPath := filepath.Join(configDir, "telegram-session.json")

	for _, d := range []string{configDir, mediaDir} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			log.Fatalf("[main] mkdir %s: %v\n", d, err)
		}
	}

	// --- Protocol writer (stdout) ---
	writer := protocol.NewWriter()

	// --- Signal handling ---
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Update dispatcher (handles incoming messages) ---
	dispatcher := tg.NewUpdateDispatcher()

	// --- Build gotd client ---
	tgc := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: &telegram.FileSessionStorage{Path: sessionPath},
		UpdateHandler:  dispatcher,
	})

	client := &tgClient{
		tg:       tgc,
		writer:   writer,
		mediaDir: mediaDir,
	}

	// Wire the incoming-message handler.
	dispatcher.OnNewMessage(func(mctx context.Context, _ tg.Entities, u *tg.UpdateNewMessage) error {
		msg, ok := u.Message.(*tg.Message)
		if !ok || msg.Out {
			return nil
		}
		chatID := peerToChatID(msg.PeerID)
		pm := buildIncomingMessage(msg, chatID)
		if err := writer.SendTyped("message.new", "", pm); err != nil {
			log.Printf("[update] emit message.new: %v\n", err)
		}
		_ = writer.SendTyped("notification", "", protocol.Notification{
			Title:   "Telegram",
			Body:    pm.Text,
			Service: "telegram",
		})
		return nil
	})

	// --- Run client ---
	runErr := make(chan error, 1)
	go func() {
		runErr <- tgc.Run(ctx, func(runCtx context.Context) error {
			// Emit initial status.
			_ = writer.SendTyped("status", "", protocol.StatusData{Status: "connected"})
			log.Println("[main] Telegram client connected")

			// Check if already authenticated.
			status, err := tgc.Auth().Status(runCtx)
			if err == nil && status.Authorized {
				self, err := tgc.Self(runCtx)
				if err == nil {
					name := self.FirstName
					if self.LastName != "" {
						name += " " + self.LastName
					}
					_ = writer.SendTyped("auth.success", "", protocol.AuthSuccess{
						User:  name,
						Phone: self.Phone,
					})
					log.Printf("[main] already authenticated as %s\n", name)
				}
			} else {
				_ = writer.SendTyped("status", "", protocol.StatusData{Status: "auth_needed"})
			}

			// Block until context is cancelled (stdin goroutine drives the work).
			<-runCtx.Done()
			return nil
		})
	}()

	// --- Stdin command loop ---
	reader := protocol.NewReader()
	stdinDone := make(chan struct{})
	go func() {
		defer close(stdinDone)
		for {
			env, err := reader.Read()
			if err != nil {
				if err.Error() != "EOF" {
					log.Printf("[stdin] read error: %v\n", err)
				}
				stop() // EOF on stdin → clean shutdown
				return
			}
			handleCommand(ctx, client, env)
		}
	}()

	// Wait for shutdown signal or stdin EOF.
	select {
	case <-ctx.Done():
		log.Println("[main] shutting down…")
	case err := <-runErr:
		if err != nil {
			log.Printf("[main] client run error: %v\n", err)
		}
	}

	_ = writer.SendTyped("status", "", protocol.StatusData{Status: "disconnected"})
}

// handleCommand dispatches a single protocol envelope to the correct handler.
func handleCommand(ctx context.Context, client *tgClient, env protocol.Envelope) {
	switch env.Type {
	case "auth.start":
		// Signal that we need a phone number to begin.
		_ = client.writer.SendTyped("auth.phone_needed", env.ID, nil)

	case "auth.phone":
		var req protocol.AuthStart
		if err := protocol.ParseData(env, &req); err != nil {
			log.Printf("[cmd] parse auth.phone: %v\n", err)
			return
		}
		handleAuthStart(ctx, client, env.ID, req)

	case "auth.code":
		var req protocol.AuthCode
		if err := protocol.ParseData(env, &req); err != nil {
			log.Printf("[cmd] parse auth.code: %v\n", err)
			return
		}
		handleAuthCode(client, req)

	case "chats.list":
		go handleChatsList(ctx, client, client.writer, env.ID)

	case "chat.messages":
		var req protocol.ChatMessagesRequest
		if err := protocol.ParseData(env, &req); err != nil {
			log.Printf("[cmd] parse chat.messages: %v\n", err)
			return
		}
		go handleChatMessages(ctx, client, client.writer, env.ID, req)

	case "message.send":
		var req protocol.SendMessageRequest
		if err := protocol.ParseData(env, &req); err != nil {
			log.Printf("[cmd] parse message.send: %v\n", err)
			return
		}
		go handleSendMessage(ctx, client, client.writer, env.ID, req)

	default:
		log.Printf("[cmd] unknown type: %s\n", env.Type)
		_ = client.writer.SendTyped("error", env.ID, map[string]string{
			"message": fmt.Sprintf("unknown command: %s", env.Type),
		})
	}
}

// handleAuthStart kicks off the authentication flow in a goroutine.
func handleAuthStart(ctx context.Context, client *tgClient, id string, req protocol.AuthStart) {
	if req.Phone == "" {
		_ = client.writer.SendTyped("error", id, map[string]string{"message": "phone is required for auth.start"})
		return
	}
	af := newAuthFlow(client.writer)
	af.phone = req.Phone
	client.af = af
	go runAuthFlow(ctx, client, af, id)
}

// handleAuthCode forwards the verification code to a waiting auth flow.
func handleAuthCode(client *tgClient, req protocol.AuthCode) {
	if client.af == nil {
		log.Println("[auth] received auth.code but no flow is in progress")
		return
	}
	client.af.submitCode(req.Code)
}

// loadCredentials reads TELEGRAM_API_ID and TELEGRAM_API_HASH from env,
// falling back to ~/.config/switchboard/telegram.env if either is missing.
func loadCredentials() (int, string, error) {
	apiIDStr := os.Getenv("TELEGRAM_API_ID")
	apiHash := os.Getenv("TELEGRAM_API_HASH")

	if apiIDStr == "" || apiHash == "" {
		envFile := filepath.Join(os.Getenv("HOME"), ".config", "switchboard", "telegram.env")
		vars, err := parseEnvFile(envFile)
		if err != nil && !os.IsNotExist(err) {
			log.Printf("[creds] could not read %s: %v\n", envFile, err)
		}
		if v, ok := vars["TELEGRAM_API_ID"]; ok && apiIDStr == "" {
			apiIDStr = v
		}
		if v, ok := vars["TELEGRAM_API_HASH"]; ok && apiHash == "" {
			apiHash = v
		}
	}

	if apiIDStr == "" {
		return 0, "", fmt.Errorf("TELEGRAM_API_ID not set (env or ~/.config/switchboard/telegram.env)")
	}
	if apiHash == "" {
		return 0, "", fmt.Errorf("TELEGRAM_API_HASH not set (env or ~/.config/switchboard/telegram.env)")
	}

	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil {
		return 0, "", fmt.Errorf("TELEGRAM_API_ID %q is not an integer: %w", apiIDStr, err)
	}
	return apiID, apiHash, nil
}

// parseEnvFile reads a KEY=VALUE file and returns a map.
// Lines starting with '#' and blank lines are ignored.
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Strip optional surrounding quotes.
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
			(val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		if key != "" {
			out[key] = val
		}
	}
	return out, scanner.Err()
}

