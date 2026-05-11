package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Cipher seals and opens byte slices with AES-256-GCM. The 12-byte nonce
// is generated fresh per call and prepended to the ciphertext on output.
type Cipher struct {
	aead cipher.AEAD
}

// NewCipherFromBase64 constructs a Cipher from a base64-encoded 32-byte key.
// We require exactly 32 bytes so it's always AES-256, no ambiguity.
func NewCipherFromBase64(b64 string) (*Cipher, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("decode base64 key: %w", err)
	}
	return NewCipher(raw)
}

// NewCipher constructs a Cipher from a raw 32-byte key.
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes.NewCipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher.NewGCM: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Seal encrypts plaintext, returning nonce||ciphertext||tag.
func (c *Cipher) Seal(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read nonce: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Open reverses Seal. It fails on a tag mismatch (tampering or wrong key).
func (c *Cipher) Open(blob []byte) ([]byte, error) {
	ns := c.aead.NonceSize()
	if len(blob) < ns {
		return nil, errors.New("ciphertext shorter than nonce")
	}
	nonce, ct := blob[:ns], blob[ns:]
	pt, err := c.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("aead open: %w", err)
	}
	return pt, nil
}

// SealString is a convenience wrapper for string plaintexts.
func (c *Cipher) SealString(s string) ([]byte, error) { return c.Seal([]byte(s)) }

// OpenString is a convenience wrapper that returns the plaintext as a string.
func (c *Cipher) OpenString(blob []byte) (string, error) {
	pt, err := c.Open(blob)
	if err != nil {
		return "", err
	}
	return string(pt), nil
}
