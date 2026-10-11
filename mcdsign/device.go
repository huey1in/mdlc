package mcdsign

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// NewDeviceID returns a random logical device ID as 32 lowercase hexadecimal
// characters. Generate it once per account, persist it in the calling
// application, and reuse it for SMS, login, and subsequent requests.
func NewDeviceID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("mcdsign: generate device ID: %w", err)
	}
	return hex.EncodeToString(data[:]), nil
}
