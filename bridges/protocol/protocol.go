package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Envelope is the top-level JSON-lines message format.
type Envelope struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Chat represents a conversation.
type Chat struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	UnreadCount int    `json:"unread"`
	LastMessage string `json:"last_message,omitempty"`
	LastTime    int64  `json:"last_time,omitempty"`
	IsGroup     bool   `json:"is_group"`
}

// Message represents a single message in a conversation.
type Message struct {
	ID        string `json:"id"`
	ChatID    string `json:"chat_id"`
	From      string `json:"from"`
	FromMe    bool   `json:"from_me"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
	ImagePath string `json:"image_path,omitempty"`
}

// AuthQR is emitted when a QR code is available for scanning.
type AuthQR struct {
	Code string `json:"code"`
}

// AuthCodeNeeded is emitted when a phone code is needed.
type AuthCodeNeeded struct {
	PhoneHint string `json:"phone_hint"`
}

// AuthSuccess is emitted when authentication completes.
type AuthSuccess struct {
	User  string `json:"user"`
	Phone string `json:"phone,omitempty"`
}

// AuthStart is received to begin authentication.
type AuthStart struct {
	Phone string `json:"phone,omitempty"`
}

// AuthCode is received with the verification code.
type AuthCode struct {
	Code string `json:"code"`
}

// ChatListRequest is for requesting chats.
type ChatListRequest struct{}

// ChatMessagesRequest is for requesting messages in a chat.
type ChatMessagesRequest struct {
	ChatID string `json:"chat_id"`
	Limit  int    `json:"limit"`
}

// SendMessageRequest is for sending a message.
type SendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// ChatListResponse wraps the list of chats.
type ChatListResponse struct {
	Chats []Chat `json:"chats"`
}

// ChatMessagesResponse wraps the list of messages.
type ChatMessagesResponse struct {
	Messages []Message `json:"messages"`
}

// Notification is emitted for OS notifications.
type Notification struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	Service string `json:"service"`
}

// StatusData wraps bridge status.
type StatusData struct {
	Status string `json:"status"` // "connected", "disconnected", "auth_needed"
}

// Writer writes JSON-lines to stdout.
type Writer struct {
	w io.Writer
}

// NewWriter creates a Writer that writes to stdout.
func NewWriter() *Writer {
	return &Writer{w: os.Stdout}
}

// Send marshals an envelope and writes it as a JSON line.
func (w *Writer) Send(env Envelope) error {
	if env.Data == nil {
		env.Data = json.RawMessage("null")
	}
	b, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	_, err = fmt.Fprintf(w.w, "%s\n", b)
	return err
}

// SendTyped marshals the data and sends an envelope with the given type.
func (w *Writer) SendTyped(msgType string, id string, data any) error {
	var raw json.RawMessage
	if data != nil {
		b, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal data: %w", err)
		}
		raw = b
	}
	return w.Send(Envelope{Type: msgType, ID: id, Data: raw})
}

// Reader reads JSON-lines from stdin.
type Reader struct {
	scanner *bufio.Scanner
}

// NewReader creates a Reader that reads from stdin.
func NewReader() *Reader {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer
	return &Reader{scanner: scanner}
}

// Read blocks until the next JSON-line is available, then parses it.
func (r *Reader) Read() (Envelope, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return Envelope{}, fmt.Errorf("scan: %w", err)
		}
		return Envelope{}, io.EOF
	}
	var env Envelope
	if err := json.Unmarshal(r.scanner.Bytes(), &env); err != nil {
		return Envelope{}, fmt.Errorf("unmarshal: %w", err)
	}
	return env, nil
}

// ParseData unmarshals the Data field of an envelope into the target.
func ParseData(env Envelope, target any) error {
	if env.Data == nil {
		return nil
	}
	return json.Unmarshal(env.Data, target)
}
