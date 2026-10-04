package argon2id

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Hasher struct {
	Memory, Iterations    uint32
	Parallelism           uint8
	SaltLength, KeyLength uint32
}

func Default() Hasher { return Hasher{64 * 1024, 3, 2, 16, 32} }

func (h Hasher) Hash(password string) (string, error) {
	if err := h.validate(); err != nil {
		return "", err
	}
	salt := make([]byte, h.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, h.Iterations, h.Memory, h.Parallelism, h.KeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", h.Memory, h.Iterations, h.Parallelism, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func (h Hasher) Verify(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false, errors.New("invalid argon2id encoding")
	}
	values := map[string]uint64{}
	for _, item := range strings.Split(parts[3], ",") {
		pair := strings.SplitN(item, "=", 2)
		if len(pair) != 2 {
			return false, errors.New("invalid argon2id parameters")
		}
		value, err := strconv.ParseUint(pair[1], 10, 32)
		if err != nil {
			return false, errors.New("invalid argon2id parameters")
		}
		values[pair[0]] = value
	}
	if values["m"] == 0 || values["t"] == 0 || values["p"] == 0 || values["p"] > 255 {
		return false, errors.New("invalid argon2id parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, errors.New("invalid argon2id salt")
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errors.New("invalid argon2id hash")
	}
	got := argon2.IDKey([]byte(password), salt, uint32(values["t"]), uint32(values["m"]), uint8(values["p"]), uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

func (h Hasher) validate() error {
	if h.Memory == 0 || h.Iterations == 0 || h.Parallelism == 0 || h.SaltLength < 16 || h.KeyLength < 16 {
		return errors.New("invalid argon2id configuration")
	}
	return nil
}
