package swordpure

import (
	"bytes"
	"testing"
)

func TestSapphireCipher_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		plaintext string
	}{
		{
			name:      "simple key",
			key:       "testkey",
			plaintext: "Hello, World!",
		},
		{
			name:      "SWORD-style key",
			key:       "SpaRV1909",
			plaintext: "In the beginning God created the heaven and the earth.",
		},
		{
			name:      "empty plaintext",
			key:       "key",
			plaintext: "",
		},
		{
			name:      "long plaintext",
			key:       "secretkey123",
			plaintext: "This is a longer piece of text that will span multiple blocks and test the cipher's ability to handle larger data correctly.",
		},
		{
			name:      "binary data",
			key:       "binkey",
			plaintext: string([]byte{0x00, 0x01, 0x02, 0x03, 0xFE, 0xFF}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			encCipher := NewSapphireCipher([]byte(tt.key))
			encrypted := make([]byte, len(tt.plaintext))
			copy(encrypted, tt.plaintext)
			encCipher.Encrypt(encrypted)

			// Decrypt
			decCipher := NewSapphireCipher([]byte(tt.key))
			decrypted := make([]byte, len(encrypted))
			copy(decrypted, encrypted)
			decCipher.Decrypt(decrypted)

			if string(decrypted) != tt.plaintext {
				t.Errorf("roundtrip failed: got %q, want %q", string(decrypted), tt.plaintext)
			}
		})
	}
}

func TestSapphireCipher_DecryptCopy(t *testing.T) {
	key := []byte("testkey")
	plaintext := []byte("test message")

	// Encrypt
	encCipher := NewSapphireCipher(key)
	encrypted := make([]byte, len(plaintext))
	copy(encrypted, plaintext)
	encCipher.Encrypt(encrypted)

	// Decrypt with copy
	decCipher := NewSapphireCipher(key)
	decrypted := decCipher.DecryptCopy(encrypted)

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("DecryptCopy failed: got %q, want %q", string(decrypted), string(plaintext))
	}

	// Original should be unchanged
	encCipher2 := NewSapphireCipher(key)
	encrypted2 := make([]byte, len(plaintext))
	copy(encrypted2, plaintext)
	encCipher2.Encrypt(encrypted2)
	if !bytes.Equal(encrypted, encrypted2) {
		t.Error("DecryptCopy modified the original data")
	}
}

func TestSapphireCipher_DifferentKeys(t *testing.T) {
	plaintext := []byte("secret message")

	cipher1 := NewSapphireCipher([]byte("key1"))
	cipher2 := NewSapphireCipher([]byte("key2"))

	encrypted1 := make([]byte, len(plaintext))
	encrypted2 := make([]byte, len(plaintext))
	copy(encrypted1, plaintext)
	copy(encrypted2, plaintext)

	cipher1.Encrypt(encrypted1)
	cipher2.Encrypt(encrypted2)

	if bytes.Equal(encrypted1, encrypted2) {
		t.Error("different keys produced same ciphertext")
	}
}

func TestSapphireCipher_EmptyKey(t *testing.T) {
	cipher := NewSapphireCipher([]byte{})
	plaintext := []byte("test")
	encrypted := make([]byte, len(plaintext))
	copy(encrypted, plaintext)
	cipher.Encrypt(encrypted)

	// Empty key should still produce some transformation
	// (identity permutation but with state updates)
	cipher2 := NewSapphireCipher([]byte{})
	cipher2.Decrypt(encrypted)

	if !bytes.Equal(encrypted, plaintext) {
		t.Errorf("empty key roundtrip failed: got %q, want %q", string(encrypted), string(plaintext))
	}
}

func TestSapphireCipher_Reset(t *testing.T) {
	key := []byte("testkey")
	plaintext := []byte("test message")

	cipher := NewSapphireCipher(key)
	encrypted1 := make([]byte, len(plaintext))
	copy(encrypted1, plaintext)
	cipher.Encrypt(encrypted1)

	// Reset and encrypt again
	cipher.Reset(key)
	encrypted2 := make([]byte, len(plaintext))
	copy(encrypted2, plaintext)
	cipher.Encrypt(encrypted2)

	if !bytes.Equal(encrypted1, encrypted2) {
		t.Error("Reset did not restore cipher to initial state")
	}
}

func TestSapphireCipher_StreamProperty(t *testing.T) {
	// Stream cipher: encrypting byte by byte should give same result as all at once
	key := []byte("streamtest")
	plaintext := []byte("test message for stream")

	// Encrypt all at once
	cipher1 := NewSapphireCipher(key)
	allAtOnce := make([]byte, len(plaintext))
	copy(allAtOnce, plaintext)
	cipher1.Encrypt(allAtOnce)

	// Encrypt byte by byte
	cipher2 := NewSapphireCipher(key)
	byteByByte := make([]byte, len(plaintext))
	for i, b := range plaintext {
		byteByByte[i] = cipher2.encryptByte(b)
	}

	if !bytes.Equal(allAtOnce, byteByByte) {
		t.Error("byte-by-byte encryption differs from batch encryption")
	}
}
