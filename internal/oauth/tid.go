package oauth

import (
	"time"
)

// ATProto uses a sortable base32 alphabet where digits come first
// This ensures lexicographic ordering matches numerical ordering
const tidAlphabet = "234567abcdefghijklmnopqrstuvwxyz"

// GenerateTID generates a timestamp-based identifier (TID) for ATProto records
// TID format: base32-sortable encoded microsecond timestamp (13 characters)
// This ensures records are sortable by creation time
func GenerateTID() string {
	// Get current time in microseconds since Unix epoch
	now := time.Now()
	microTimestamp := uint64(now.UnixMicro())

	// Encode as base32-sortable (13 characters for 64-bit timestamp)
	// Each character encodes 5 bits, 13 chars = 65 bits (enough for 64-bit timestamp)
	result := make([]byte, 13)
	for i := 12; i >= 0; i-- {
		result[i] = tidAlphabet[microTimestamp&0x1f]
		microTimestamp >>= 5
	}

	return string(result)
}
