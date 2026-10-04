package tokenhmac

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
)

type Deriver struct{ secret []byte }

func New(secret []byte) (*Deriver, error) {
	if len(secret) < 32 {
		return nil, errors.New("identity token secret must be at least 32 bytes")
	}
	copySecret := append([]byte(nil), secret...)
	return &Deriver{copySecret}, nil
}

func (d *Deriver) Derive(tokenID, accountID, purpose string) (string, []byte, error) {
	if tokenID == "" || accountID == "" || purpose == "" {
		return "", nil, errors.New("identity token derivation inputs are required")
	}
	mac := hmac.New(sha256.New, d.secret)
	_, _ = mac.Write([]byte("identity-action-v1\x00" + tokenID + "\x00" + accountID + "\x00" + purpose))
	value := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	hash := sha256.Sum256([]byte(value))
	return value, hash[:], nil
}
