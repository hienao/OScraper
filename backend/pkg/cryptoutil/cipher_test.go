package cryptoutil

import "testing"

func TestCipherRoundTrip(t *testing.T) {
	cipher, err := New("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := cipher.Encrypt("openlist-token")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "openlist-token" {
		t.Fatal("ciphertext must not equal plaintext")
	}
	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "openlist-token" {
		t.Fatalf("unexpected plaintext: %q", decrypted)
	}
}

func TestCipherRejectsInvalidKey(t *testing.T) {
	if _, err := New("short"); err == nil {
		t.Fatal("expected invalid key error")
	}
}
