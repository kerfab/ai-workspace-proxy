package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseEncryptionKeyRequiresStrongKeyMaterial(t *testing.T) {
	raw32 := strings.Repeat("a", 32)
	hex32 := strings.Repeat("0f", 32)
	base64Key := base64.StdEncoding.EncodeToString([]byte(raw32))

	for _, value := range []string{raw32, hex32, base64Key} {
		key, err := parseEncryptionKey(value)
		if err != nil {
			t.Fatalf("expected valid key %q: %v", value, err)
		}
		if len(key) != 32 {
			t.Fatalf("expected 32-byte key for %q, got %d", value, len(key))
		}
	}

	if _, err := parseEncryptionKey("weak"); err == nil {
		t.Fatal("expected weak key material to be rejected")
	}
}
