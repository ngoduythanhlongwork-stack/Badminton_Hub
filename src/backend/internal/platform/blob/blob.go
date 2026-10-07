package blob

import (
	"context"
	"errors"
	"io"
	"time"
)

var ErrUnavailable = errors.New("object storage is unavailable")

type Store interface {
	Put(context.Context, string, io.Reader, int64, string) error
	SignedReadURL(context.Context, string, time.Duration) (string, error)
	Delete(context.Context, string) error
}

// Disabled fails closed until an operator selects and configures a provider.
type Disabled struct{}

func (Disabled) Put(context.Context, string, io.Reader, int64, string) error { return ErrUnavailable }
func (Disabled) SignedReadURL(context.Context, string, time.Duration) (string, error) {
	return "", ErrUnavailable
}
func (Disabled) Delete(context.Context, string) error { return ErrUnavailable }
