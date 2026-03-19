package session

import (
	"crypto/rand"
	"encoding/hex"
)

func generateId(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
