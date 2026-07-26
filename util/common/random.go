package common

import (
	crand "crypto/rand"
	"io"
	"math/big"
)

var (
	allSeq = []rune{
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
	}

	// randomReader is crypto/rand.Reader in production. Keeping it package-local
	// makes entropy-source failures testable without changing callers.
	randomReader io.Reader = crand.Reader
)

// RandomWithError returns a cryptographically secure random alphanumeric string.
// It returns an error if the system entropy source cannot provide random data.
func RandomWithError(n int) (string, error) {
	if n <= 0 || len(allSeq) == 0 {
		return "", nil
	}

	result := make([]rune, n)
	max := big.NewInt(int64(len(allSeq)))
	for i := range result {
		value, err := crand.Int(randomReader, max)
		if err != nil {
			return "", err
		}
		result[i] = allSeq[value.Int64()]
	}
	return string(result), nil
}

// Random is retained for compatibility. It never falls back to math/rand; it
// returns an empty string when secure randomness is unavailable. Security-
// sensitive callers must use RandomWithError so they can handle that failure.
func Random(n int) string {
	result, _ := RandomWithError(n)
	return result
}

// RandomIntWithError returns a cryptographically secure random integer in [0, n).
// It returns an error if the system entropy source cannot provide random data.
func RandomIntWithError(n int) (int, error) {
	if n <= 0 {
		return 0, nil
	}

	result, err := crand.Int(randomReader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(result.Int64()), nil
}

// RandomInt is retained for compatibility. It never falls back to math/rand;
// it returns zero when secure randomness is unavailable. Callers that need to
// distinguish zero from an entropy failure must use RandomIntWithError.
func RandomInt(n int) int {
	result, _ := RandomIntWithError(n)
	return result
}

// RandomBytes returns n cryptographically secure random bytes. It returns an
// error rather than producing a weaker fallback when the entropy source fails.
func RandomBytes(n int) ([]byte, error) {
	if n <= 0 {
		return []byte{}, nil
	}

	result := make([]byte, n)
	if _, err := io.ReadFull(randomReader, result); err != nil {
		return nil, err
	}
	return result, nil
}
