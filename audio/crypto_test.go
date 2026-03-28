package audio

import (
	"bytes"
	"testing"
)

func TestOCBEncryptDecrypt(t *testing.T) {
	// Test key from the Python tests
	rawKey := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
	nonce := []byte{
		0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88,
		0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00,
	}

	cs := NewCryptStateOCB2()
	if err := cs.SetKey(rawKey, nonce, nonce); err != nil {
		t.Fatalf("SetKey failed: %v", err)
	}

	// Test various lengths
	for ll := 1; ll < 128; ll++ {
		src := make([]byte, ll)
		for i := range src {
			src[i] = byte(i + 1)
		}

		encrypted, err := cs.Encrypt(src)
		if err != nil {
			t.Fatalf("Encrypt failed for length %d: %v", ll, err)
		}

		// Decrypt with fresh state
		cs2 := NewCryptStateOCB2()
		cs2.SetKey(rawKey, nonce, nonce)

		decrypted, err := cs2.Decrypt(encrypted, ll)
		if err != nil {
			t.Fatalf("Decrypt failed for length %d: %v", ll, err)
		}

		if !bytes.Equal(decrypted, src) {
			t.Errorf("Decrypt mismatch for length %d", ll)
		}
	}
}

func TestOCBDecryptReplayAttack(t *testing.T) {
	rawKey := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f,
	}
	nonce := []byte{
		0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, 0x88,
		0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, 0x00,
	}

	enc := NewCryptStateOCB2()
	enc.SetKey(rawKey, nonce, nonce)

	dec := NewCryptStateOCB2()
	dec.SetKey(rawKey, nonce, nonce)

	secret := []byte("abcdefghi")

	// Encrypt a packet
	crypted, err := enc.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// First decrypt should succeed
	_, err = dec.Decrypt(crypted, len(secret))
	if err != nil {
		t.Fatalf("First decrypt should succeed: %v", err)
	}

	// Second decrypt with SAME state should fail (replay)
	_, err = dec.Decrypt(crypted, len(secret))
	if err == nil {
		t.Errorf("Second decrypt should fail (replay attack)")
	}
}

func TestS2Doubling(t *testing.T) {
	block := []byte{
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80,
	}

	result := s2(block)
	if result[14] != 0x01 {
		t.Errorf("Expected result[14] = 0x01, got 0x%02x", result[14])
	}
	if result[15] != 0x00 {
		t.Errorf("Expected result[15] = 0x00, got 0x%02x", result[15])
	}
}

func TestAESBlockEncrypt(t *testing.T) {
	key := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	plaintext := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77,
		0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}

	cs := NewCryptStateOCB2()
	cs.SetKey(key, key, key)

	decrypted := make([]byte, 16)
	cs.aes.Decrypt(decrypted, plaintext)

	// Verify it's not identity
	if bytes.Equal(decrypted, plaintext) {
		t.Errorf("AES should change the data")
	}

	// Verify decrypt round-trips correctly
	cs2 := NewCryptStateOCB2()
	cs2.SetKey(key, key, key)
	encrypted := make([]byte, 16)
	cs2.aes.Encrypt(encrypted, plaintext)
	cs2.aes.Decrypt(decrypted, encrypted)
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("AES round-trip failed")
	}
}
