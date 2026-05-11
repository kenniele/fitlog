package auth

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return k
}

func TestCipher_RoundTrip(t *testing.T) {
	c, err := NewCipher(newTestKey(t))
	require.NoError(t, err)

	plain := []byte("whoop-refresh-token-xyz-789")
	blob, err := c.Seal(plain)
	require.NoError(t, err)
	require.NotEqual(t, plain, blob, "ciphertext must differ from plaintext")

	out, err := c.Open(blob)
	require.NoError(t, err)
	require.True(t, bytes.Equal(plain, out))
}

func TestCipher_DifferentNoncePerCall(t *testing.T) {
	c, err := NewCipher(newTestKey(t))
	require.NoError(t, err)

	a, err := c.Seal([]byte("same"))
	require.NoError(t, err)
	b, err := c.Seal([]byte("same"))
	require.NoError(t, err)

	require.False(t, bytes.Equal(a, b), "two seals of identical plaintext must produce different output")
}

func TestCipher_TamperDetected(t *testing.T) {
	c, err := NewCipher(newTestKey(t))
	require.NoError(t, err)

	blob, err := c.Seal([]byte("hello"))
	require.NoError(t, err)

	tampered := append([]byte(nil), blob...)
	tampered[len(tampered)-1] ^= 0x01

	_, err = c.Open(tampered)
	require.Error(t, err)
}

func TestCipher_WrongKeyFails(t *testing.T) {
	c1, err := NewCipher(newTestKey(t))
	require.NoError(t, err)
	c2, err := NewCipher(newTestKey(t))
	require.NoError(t, err)

	blob, err := c1.Seal([]byte("secret"))
	require.NoError(t, err)

	_, err = c2.Open(blob)
	require.Error(t, err)
}

func TestNewCipher_RejectsWrongKeyLength(t *testing.T) {
	_, err := NewCipher(make([]byte, 16))
	require.Error(t, err)
	_, err = NewCipher(make([]byte, 0))
	require.Error(t, err)
}

func TestNewCipherFromBase64(t *testing.T) {
	key := newTestKey(t)
	b64 := base64.StdEncoding.EncodeToString(key)

	c, err := NewCipherFromBase64(b64)
	require.NoError(t, err)

	blob, err := c.SealString("hi")
	require.NoError(t, err)
	out, err := c.OpenString(blob)
	require.NoError(t, err)
	require.Equal(t, "hi", out)
}

func TestNewCipherFromBase64_BadInput(t *testing.T) {
	_, err := NewCipherFromBase64("!!!not-base64!!!")
	require.Error(t, err)
}

func TestCipher_Open_ShorterThanNonce(t *testing.T) {
	c, err := NewCipher(newTestKey(t))
	require.NoError(t, err)

	_, err = c.Open([]byte{0x01, 0x02})
	require.Error(t, err)
}
