package gateway

import (
	"testing"
	"time"
)

func TestRequestSignatureRejectsTamperReplayAndSkew(t *testing.T) {
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	timestamp := now.Format(time.RFC3339Nano)
	body := []byte("frame")
	signature := RequestSignature("identity", "POST", "/api/agent/v1/checkin", timestamp, "nonce-0123456789", 7, body)
	if !VerifyRequestSignature("identity", "POST", "/api/agent/v1/checkin", timestamp, "nonce-0123456789", 7, body, signature, now) {
		t.Fatal("valid signature rejected")
	}
	for _, changed := range []struct {
		secret, path, nonce, signature string
		sequence                       uint64
		body                           []byte
		now                            time.Time
	}{
		{"wrong", "/api/agent/v1/checkin", "nonce-0123456789", signature, 7, body, now},
		{"identity", "/api/agent/v1/results", "nonce-0123456789", signature, 7, body, now},
		{"identity", "/api/agent/v1/checkin", "other-nonce-1234", signature, 7, body, now},
		{"identity", "/api/agent/v1/checkin", "nonce-0123456789", signature, 8, body, now},
		{"identity", "/api/agent/v1/checkin", "nonce-0123456789", signature, 7, []byte("tampered"), now},
		{"identity", "/api/agent/v1/checkin", "nonce-0123456789", signature, 7, body, now.Add(ClockSkewTolerance + time.Second)},
	} {
		if VerifyRequestSignature(changed.secret, "POST", changed.path, timestamp, changed.nonce, changed.sequence, changed.body, changed.signature, changed.now) {
			t.Fatal("changed request accepted")
		}
	}
}
