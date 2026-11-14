package jobs

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func newID(prefix string) string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
	}
	return prefix + hex.EncodeToString(value[:])
}

// BookingIdempotencyKey scopes retries to a single call and a normalized set
// of booking details without storing addresses or phone numbers in the key.
func BookingIdempotencyKey(scope string, request BookingRequest) string {
	parts := []string{
		strings.ToLower(strings.TrimSpace(request.Service)),
		strings.ToLower(strings.TrimSpace(request.Date)),
		strings.ToLower(strings.TrimSpace(request.Window)),
		strings.ToLower(strings.TrimSpace(request.Address)),
		NormalizePhone(request.Phone),
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return strings.TrimSpace(scope) + "/booking/" + hex.EncodeToString(digest[:])
}
