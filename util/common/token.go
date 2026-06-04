package common

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strings"
)

const tokenHashPrefix = "sha256:"

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return tokenHashPrefix + hex.EncodeToString(sum[:])
}

func IsTokenHash(token string) bool {
	return strings.HasPrefix(token, tokenHashPrefix)
}

func CheckToken(storedToken string, token string) bool {
	if storedToken == "" || token == "" {
		return false
	}
	if IsTokenHash(storedToken) {
		hashedToken := HashToken(token)
		return subtle.ConstantTimeCompare([]byte(storedToken), []byte(hashedToken)) == 1
	}
	return subtle.ConstantTimeCompare([]byte(storedToken), []byte(token)) == 1
}
