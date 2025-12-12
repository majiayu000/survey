package oauth

import (
	"testing"
	"time"
)

func TestGenerateTID(t *testing.T) {
	t.Run("generates valid TID", func(t *testing.T) {
		tid := GenerateTID()

		// TID should be 13 characters (base32-encoded microsecond timestamp)
		if len(tid) != 13 {
			t.Errorf("Expected TID length 13, got %d: %s", len(tid), tid)
		}

		// Should only contain base32 characters (2-7, a-z)
		for _, c := range tid {
			if !((c >= '2' && c <= '7') || (c >= 'a' && c <= 'z')) {
				t.Errorf("TID contains invalid character %c: %s", c, tid)
			}
		}
	})

	t.Run("generates unique TIDs", func(t *testing.T) {
		tid1 := GenerateTID()
		time.Sleep(1 * time.Millisecond) // Ensure time advances
		tid2 := GenerateTID()

		if tid1 == tid2 {
			t.Errorf("Expected unique TIDs, got duplicate: %s", tid1)
		}
	})

	t.Run("TIDs are monotonically increasing", func(t *testing.T) {
		tid1 := GenerateTID()
		time.Sleep(1 * time.Millisecond)
		tid2 := GenerateTID()

		// Lexicographically, later TIDs should be > earlier TIDs
		if tid2 <= tid1 {
			t.Errorf("Expected tid2 > tid1, got tid1=%s, tid2=%s", tid1, tid2)
		}
	})
}
