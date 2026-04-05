package id

import (
	"crypto/rand"
	"encoding/hex"
)

// NewChatCmplID returns a new unique OpenAI-style completion ID.
// Format: "chatcmpl-" + 24 lowercase hex characters (12 random bytes).
func NewChatCmplID() string {
	return "chatcmpl-" + randHex(12)
}

// NewMsgID returns a new unique Anthropic-style message ID.
// Format: "msg_" + 24 lowercase hex characters (12 random bytes).
func NewMsgID() string {
	return "msg_" + randHex(12)
}

// randHex returns n random bytes encoded as lowercase hex (2n characters).
// Panics if the system CSPRNG fails (unrecoverable).
func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("id: crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}
