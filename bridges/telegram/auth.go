package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/aigustalabs/switchboard/bridges/protocol"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// authFlow manages the interactive authentication state machine.
// Because gotd/td's auth.Flow expects synchronous callbacks but our
// commands arrive asynchronously over stdin, we thread channels through.
type authFlow struct {
	// codeCh receives the SMS/app code after auth.start sends auth.code_needed.
	codeCh chan string
	// phone is set by handleAuthStart before launching the flow goroutine.
	phone string
	// password is the optional 2FA password; empty means no 2FA.
	password string
	// writer emits protocol envelopes to the host process.
	writer *protocol.Writer
}

// newAuthFlow creates an authFlow ready to run.
func newAuthFlow(writer *protocol.Writer) *authFlow {
	return &authFlow{
		codeCh: make(chan string, 1),
		writer: writer,
	}
}

// AcceptTermsOfService satisfies auth.UserAuthenticator; we just accept.
func (f *authFlow) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	log.Printf("[auth] accepting ToS (id=%s)\n", tos.ID.Data)
	return nil
}

// SignUp is called when the account does not exist yet. We reject this path
// because Switchboard is meant to attach to an existing account.
func (f *authFlow) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign-up not supported; please register via the Telegram app first")
}

// Phone returns the stored phone number.
func (f *authFlow) Phone(_ context.Context) (string, error) {
	return f.phone, nil
}

// Password returns the stored 2FA password (may be empty).
func (f *authFlow) Password(_ context.Context) (string, error) {
	return f.password, nil
}

// Code waits for the verification code that will arrive over stdin.
func (f *authFlow) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	// Notify the host that we need the code.
	if err := f.writer.SendTyped("auth.code_needed", "", protocol.AuthCodeNeeded{
		PhoneHint: f.phone,
	}); err != nil {
		return "", fmt.Errorf("send auth.code_needed: %w", err)
	}

	select {
	case code := <-f.codeCh:
		return strings.TrimSpace(code), nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// submitCode delivers a code received from the host into a waiting Code() call.
func (f *authFlow) submitCode(code string) {
	// Non-blocking send; if nothing is waiting the buffer holds it.
	select {
	case f.codeCh <- code:
	default:
		// channel already has a pending code — replace it
		<-f.codeCh
		f.codeCh <- code
	}
}

// runAuthFlow executes the full gotd auth flow for the given phone.
// It blocks until auth completes or ctx is cancelled.
func runAuthFlow(ctx context.Context, client *tgClient, flow *authFlow, id string) {
	authClient := client.tg.Auth()

	flow.phone = strings.TrimSpace(flow.phone)

	// Check if already authenticated; if so, skip the whole flow.
	status, err := authClient.Status(ctx)
	if err == nil && status.Authorized {
		log.Println("[auth] already authorized, skipping flow")
		self, err := client.tg.Self(ctx)
		if err != nil {
			log.Printf("[auth] Self() error: %v\n", err)
			return
		}
		name := self.FirstName
		if self.LastName != "" {
			name += " " + self.LastName
		}
		_ = client.writer.SendTyped("auth.success", id, protocol.AuthSuccess{
			User:  name,
			Phone: self.Phone,
		})
		return
	}

	authFlow := auth.NewFlow(flow, auth.SendCodeOptions{})
	if err := authFlow.Run(ctx, authClient); err != nil {
		log.Printf("[auth] flow error: %v\n", err)
		_ = client.writer.SendTyped("error", id, map[string]string{"message": err.Error()})
		return
	}

	self, err := client.tg.Self(ctx)
	if err != nil {
		log.Printf("[auth] Self() after auth: %v\n", err)
		return
	}

	name := self.FirstName
	if self.LastName != "" {
		name += " " + self.LastName
	}
	_ = client.writer.SendTyped("auth.success", id, protocol.AuthSuccess{
		User:  name,
		Phone: self.Phone,
	})
	log.Printf("[auth] authenticated as %s\n", name)
}
