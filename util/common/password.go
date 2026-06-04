package common

import (
	"crypto/subtle"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func IsPasswordHash(password string) bool {
	return strings.HasPrefix(password, "$2a$") ||
		strings.HasPrefix(password, "$2b$") ||
		strings.HasPrefix(password, "$2x$") ||
		strings.HasPrefix(password, "$2y$")
}

func CheckPassword(storedPassword string, password string) bool {
	if IsPasswordHash(storedPassword) {
		return bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(password)) == nil
	}
	return subtle.ConstantTimeCompare([]byte(storedPassword), []byte(password)) == 1
}
