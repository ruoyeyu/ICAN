package config

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestEncryptedConfigIntegrityAndOwnership(t *testing.T) {
	old := Cfg
	defer func() { Cfg = old }()
	Cfg.DataEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	ciphertext, err := EncryptData("api-secret", "alice:model")
	if err != nil {
		t.Fatal(err)
	}
	if plaintext, err := DecryptData(ciphertext, "alice:model"); err != nil || plaintext != "api-secret" {
		t.Fatal("roundtrip", err)
	}
	if _, err := DecryptData(ciphertext, "bob:model"); err == nil {
		t.Fatal("cross-owner ciphertext accepted")
	}
	if _, err := DecryptData("plaintext", "alice:model"); err == nil {
		t.Fatal("legacy plaintext silently accepted")
	}
	Cfg.DataEncryptionKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32))
	if _, err := DecryptData(ciphertext, "alice:model"); err == nil {
		t.Fatal("wrong key accepted")
	}
}
