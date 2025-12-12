package oauth

import (
	"encoding/base32"
	"strings"
	"time"
)

// GenerateTID generates a timestamp-based identifier (TID) for ATProto records
// TID format: base32-encoded microsecond timestamp (13 characters)
// This ensures records are sortable by creation time
func GenerateTID() string {
	// Get current time in microseconds since Unix epoch
	now := time.Now()
	microTimestamp := now.UnixMicro()

	// Convert to 8 bytes (big-endian)
	bytes := make([]byte, 8)
	bytes[0] = byte(microTimestamp >> 56)
	bytes[1] = byte(microTimestamp >> 48)
	bytes[2] = byte(microTimestamp >> 40)
	bytes[3] = byte(microTimestamp >> 32)
	bytes[4] = byte(microTimestamp >> 24)
	bytes[5] = byte(microTimestamp >> 16)
	bytes[6] = byte(microTimestamp >> 8)
	bytes[7] = byte(microTimestamp)

	// Encode as base32 (no padding) and lowercase
	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	encoded := encoder.EncodeToString(bytes)

	// ATProto uses lowercase base32
	return strings.ToLower(encoded)
}
