package common

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type failingRandomReader struct {
	err error
}

func (r failingRandomReader) Read([]byte) (int, error) {
	return 0, r.err
}

func useRandomReader(t *testing.T, reader io.Reader) {
	t.Helper()
	previous := randomReader
	randomReader = reader
	t.Cleanup(func() {
		randomReader = previous
	})
}

func TestRandomWithErrorUsesSecureReader(t *testing.T) {
	useRandomReader(t, bytes.NewReader(make([]byte, 32)))

	value, err := RandomWithError(12)
	if err != nil {
		t.Fatalf("RandomWithError() error = %v", err)
	}
	if len(value) != 12 {
		t.Fatalf("RandomWithError() length = %d, want 12", len(value))
	}
	for _, char := range value {
		if !bytes.ContainsRune([]byte(string(allSeq)), char) {
			t.Fatalf("RandomWithError() returned character %q outside the configured alphabet", char)
		}
	}
}

func TestRandomWithErrorDoesNotFallbackOnEntropyFailure(t *testing.T) {
	sentinel := errors.New("entropy unavailable")
	useRandomReader(t, failingRandomReader{err: sentinel})

	value, err := RandomWithError(12)
	if !errors.Is(err, sentinel) {
		t.Fatalf("RandomWithError() error = %v, want %v", err, sentinel)
	}
	if value != "" {
		t.Fatalf("RandomWithError() value = %q, want empty string on entropy failure", value)
	}
	if value := Random(12); value != "" {
		t.Fatalf("Random() value = %q, want empty string rather than a weak fallback", value)
	}
}

func TestRandomIntWithErrorDoesNotFallbackOnEntropyFailure(t *testing.T) {
	sentinel := errors.New("entropy unavailable")
	useRandomReader(t, failingRandomReader{err: sentinel})

	value, err := RandomIntWithError(10)
	if !errors.Is(err, sentinel) {
		t.Fatalf("RandomIntWithError() error = %v, want %v", err, sentinel)
	}
	if value != 0 {
		t.Fatalf("RandomIntWithError() value = %d, want zero on error", value)
	}
	if value := RandomInt(10); value != 0 {
		t.Fatalf("RandomInt() value = %d, want zero rather than a weak fallback", value)
	}
}

func TestRandomBytesDoesNotFallbackOnEntropyFailure(t *testing.T) {
	sentinel := errors.New("entropy unavailable")
	useRandomReader(t, failingRandomReader{err: sentinel})

	value, err := RandomBytes(16)
	if !errors.Is(err, sentinel) {
		t.Fatalf("RandomBytes() error = %v, want %v", err, sentinel)
	}
	if value != nil {
		t.Fatalf("RandomBytes() value = %x, want nil on entropy failure", value)
	}
}

func TestRandomHelpersKeepNonPositiveInputCompatible(t *testing.T) {
	if value, err := RandomWithError(0); err != nil || value != "" {
		t.Fatalf("RandomWithError(0) = (%q, %v), want (empty, nil)", value, err)
	}
	if value, err := RandomIntWithError(0); err != nil || value != 0 {
		t.Fatalf("RandomIntWithError(0) = (%d, %v), want (0, nil)", value, err)
	}
	if value, err := RandomBytes(0); err != nil || len(value) != 0 {
		t.Fatalf("RandomBytes(0) = (%x, %v), want (empty, nil)", value, err)
	}
}
