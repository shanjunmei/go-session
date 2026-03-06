package session

import (
	"crypto/rand"
	"encoding/hex"
)

// -------------------------- 工具函数：生成会话ID --------------------------
func NewSessionId(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
