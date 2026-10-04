package identity

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"badmintonhub/internal/modules/identity/internal/application"
	"badmintonhub/internal/platform/outbox"
)

func TestEmailOutboxPayloadStoresNoBearerAndHandlerIsIdempotent(t *testing.T) {
	obligation := application.EmailObligation{Kind: "VERIFY_EMAIL", Recipient: "player@example.com", TokenID: "token-1", AccountID: "account-1", Purpose: "VERIFY_EMAIL", ExpiresAt: time.Now().Add(time.Hour)}
	payload, err := json.Marshal(obligation)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), `"Token":`) {
		t.Fatalf("payload contains bearer token: %s", payload)
	}
	sender := &CaptureEmailSender{}
	handler, err := NewEmailOutboxHandler(sender, []byte("test_identity_token_secret_32_bytes_minimum"))
	if err != nil {
		t.Fatal(err)
	}
	message := outbox.Message{ID: "message-1", Topic: "identity.email", Payload: payload}
	if err := handler(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	if err := handler(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	messages := sender.Messages()
	if len(messages) != 1 || messages[0].Token == "" || messages[0].IdempotencyKey != message.ID {
		t.Fatalf("messages=%+v", messages)
	}
}
