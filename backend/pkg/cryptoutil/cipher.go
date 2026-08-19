package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

type Cipher struct{ aead cipher.AEAD }

func New(keyValue string) (*Cipher, error) {
	key := []byte(keyValue)
	if decoded, err := base64.StdEncoding.DecodeString(keyValue); err == nil && len(decoded) == 32 {
		key = decoded
	}
	if len(key) != 32 {
		return nil, errors.New("credential encryption key must be exactly 32 bytes or base64-encoded 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) Decrypt(encoded string) (string, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := c.aead.NonceSize()
	if len(sealed) < nonceSize {
		return "", errors.New("encrypted credential is truncated")
	}
	plaintext, err := c.aead.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
