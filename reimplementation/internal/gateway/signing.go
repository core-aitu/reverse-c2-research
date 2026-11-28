package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

const ClockSkewTolerance = 5 * time.Minute

func RequestSignature(secret, method, path, timestamp, nonce string, sequence uint64, body []byte) string {
	digest := sha256.Sum256(body)
	canonical := strings.Join([]string{method, path, strconv.Itoa(Version), timestamp, nonce, strconv.FormatUint(sequence, 10), hex.EncodeToString(digest[:])}, "\n")
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(canonical))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyRequestSignature(secret, method, path, timestamp, nonce string, sequence uint64, body []byte, signature string, now time.Time) bool {
	observed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || nonce == "" || len(nonce) > 128 || now.Sub(observed) > ClockSkewTolerance || observed.Sub(now) > ClockSkewTolerance {
		return false
	}
	want, err := hex.DecodeString(RequestSignature(secret, method, path, timestamp, nonce, sequence, body))
	if err != nil {
		return false
	}
	got, err := hex.DecodeString(signature)
	return err == nil && hmac.Equal(got, want)
}
