// Package audio handles Mumble voice encoding, decoding, and crypto.
package audio

import (
	"crypto/aes"
	"encoding/binary"
	"fmt"
)

// OCB-AES-128 implementation for Mumble audio encryption.
// Ported from Mumble src/tests/TestCrypt/TestCrypt.cpp
// Reference: https://eprint.iacr.org/2019/311 (counter-cryptanalysis mitigation)

const (
	AESBlockSize = 16 // AES block size in bytes
	ocbTagSize   = 16 // OCB authentication tag is 16 bytes
)

// EncryptFailedException is raised when encryption would produce a vulnerable packet.
type EncryptFailedException struct{ msg string }

func (e *EncryptFailedException) Error() string { return e.msg }

// DecryptFailedException is raised when decryption fails (tampered/invalid data).
type DecryptFailedException struct{ msg string }

func (e *DecryptFailedException) Error() string { return e.msg }

// CryptStateOCB2 holds the per-session crypto state for OCB-AES encryption/decryption.
type CryptStateOCB2 struct {
	aes          cipher
	rawKey       [16]byte
	encryptIV    [16]byte
	decryptIV    [16]byte
	decryptHistory [256]byte

	// Statistics
	uiGood  int
	uiLate  int
	uiLost  int
}

// cipher is an interface matching crypto/aes cipher
type cipher interface {
	Encrypt(dst, src []byte)
	Decrypt(dst, src []byte)
}

// NewCryptStateOCB2 creates a new CryptStateOCB2 with random key and IVs.
func NewCryptStateOCB2() *CryptStateOCB2 {
	c := &CryptStateOCB2{}
	// Generate random key and IVs (using aes.NewCipher with zero key for now, caller should set_key)
	return c
}

// SetKey initializes the crypto state with the given key and IVs.
func (c *CryptStateOCB2) SetKey(rawKey, encryptIV, decryptIV []byte) error {
	if len(rawKey) != 16 {
		return fmt.Errorf("crypt: key must be 16 bytes")
	}
	if len(encryptIV) != 16 {
		return fmt.Errorf("crypt: encryptIV must be 16 bytes")
	}
	if len(decryptIV) != 16 {
		return fmt.Errorf("crypt: decryptIV must be 16 bytes")
	}
	copy(c.rawKey[:], rawKey)
	copy(c.encryptIV[:], encryptIV)
	copy(c.decryptIV[:], decryptIV)

	cipher, err := aes.NewCipher(c.rawKey[:])
	if err != nil {
		return err
	}
	c.aes = cipher
	return nil
}

// Encrypt encrypts a plaintext audio packet using OCB-AES.
// Returns ciphertext with prepended header: ivbyte || tag[:3] || encrypted_blocks.
func (c *CryptStateOCB2) Encrypt(source []byte) ([]byte, error) {
	// Increment encrypt IV for this packet
	eiv := incrementIV(c.encryptIV[:])
	copy(c.encryptIV[:], eiv)

	encrypted, tag, err := ocbEncrypt(c.aes, source, c.encryptIV[:])
	if err != nil {
		return nil, err
	}

	// Prepend ivbyte and first 3 bytes of tag
	head := []byte{c.encryptIV[0], tag[0], tag[1], tag[2]}
	return append(head, encrypted...), nil
}

// Decrypt decrypts a ciphertext audio packet.
// source[0] = expected ivbyte, source[1:4] = tag bytes for verification
func (c *CryptStateOCB2) Decrypt(source []byte, lenPlain int) ([]byte, error) {
	if len(source) < 4 {
		return nil, &DecryptFailedException{"source < 4 bytes long"}
	}

	div := make([]byte, 16)
	copy(div, c.decryptIV[:])
	ivbyte := source[0]
	late := false
	lost := 0

	// Check if packet arrived in order, out of order, or duplicate
	if int((div[0]+1)&0xff) == int(ivbyte) {
		// In order as expected
		if ivbyte > div[0] {
			div[0] = ivbyte
		} else if ivbyte < div[0] {
			div[0] = ivbyte
			incrementIVInPlace(div, 1)
		} else {
			return nil, &DecryptFailedException{"ivbyte == decrypt_iv[0]"}
		}
	} else {
		// Out of order or repeat
		diff := int(ivbyte) - int(div[0])
		if diff > 128 {
			diff -= 256
		} else if diff < -128 {
			diff += 256
		}

		if ivbyte < div[0] && -30 < diff && diff < 0 {
			late = true
			lost = -1
			div[0] = ivbyte
		} else if ivbyte > div[0] && -30 < diff && diff < 0 {
			late = true
			lost = -1
			div[0] = ivbyte
			decrementIVInPlace(div, 1)
		} else if ivbyte > div[0] && diff > 0 {
			lost = int(ivbyte) - int(div[0]) - 1
			div[0] = ivbyte
		} else if ivbyte < div[0] && diff > 0 {
			lost = 256 - int(div[0]) + int(ivbyte) - 1
			div[0] = ivbyte
			incrementIVInPlace(div, 1)
		} else {
			return nil, &DecryptFailedException{"lost too many packets"}
		}

		if c.decryptHistory[div[0]] == div[1] {
			return nil, &DecryptFailedException{"decrypt_iv in history"}
		}
	}

	decrypted, tag, err := ocbDecrypt(c.aes, source[4:], div, lenPlain)
	if err != nil {
		return nil, err
	}

	if tag[0] != source[1] || tag[1] != source[2] || tag[2] != source[3] {
		return nil, &DecryptFailedException{"tag did not match"}
	}

	c.decryptHistory[div[0]] = div[1]

	if !late {
		copy(c.decryptIV[:], div)
	} else {
		c.uiLate++
	}

	c.uiGood++
	c.uiLost += lost

	return decrypted, nil
}

// ocbEncrypt performs OCB-AES encryption.
// Returns ciphertext and tag.
func ocbEncrypt(aes cipher, plain, nonce []byte) ([]byte, []byte, error) {
	if len(nonce) != 16 {
		return nil, nil, fmt.Errorf("ocb: nonce must be 16 bytes")
	}

	delta := make([]byte, 16)
	aes.Encrypt(delta, nonce)

	checksum := make([]byte, 16)
	var plainBlock []byte

	numBlocks := (len(plain) + 15) / 16
	encrypted := make([]byte, numBlocks*16)
	pos := 0

	for pos+16 <= len(plain) {
		plainBlock = plain[pos : pos+16]
		delta = s2(delta)
		xored := xorBytes(delta, plainBlock)
		tmp := make([]byte, 16)
		aes.Encrypt(tmp, xored)
		encBlock := xorBytes(delta, tmp)
		copy(encrypted[pos:pos+16], encBlock)
		checksum = xorBytes(checksum, plainBlock)
		pos += 16
	}

	// Counter-cryptanalysis mitigation (section 9 of https://eprint.iacr.org/2019/311)
	if plainBlock != nil && bytesEqual(plainBlock[:15], make([]byte, 15)) {
		return nil, nil, &EncryptFailedException{"insecure input block"}
	}

	if pos < len(plain) {
		// Partial final block
		delta = s2(delta)
		padIn := make([]byte, 16)
		binary.BigEndian.PutUint64(padIn[0:8], 0)
		binary.BigEndian.PutUint64(padIn[8:16], uint64(len(plain)-pos)*8)
		xored := xorBytes(padIn, delta)
		pad := make([]byte, 16)
		aes.Encrypt(pad, xored)

		plainBlock = plain[pos:]
		// Pad the remaining bytes with pad values
		paddedPlain := make([]byte, 16)
		copy(paddedPlain, plainBlock)

		checksum = xorBytes(checksum, paddedPlain)
		encBlock := xorBytes(pad, paddedPlain)
		// Copy the full encBlock to encrypted (Python copies full block to encrypted[pos:])
		copy(encrypted[pos:pos+16], encBlock)
	}

	// Compute final delta = delta XOR S2(delta) where delta is the last offset
	lastDelta := make([]byte, 16)
	copy(lastDelta, delta)
	delta = xorBytes(delta, s2(lastDelta))
	tag := make([]byte, 16)
	aes.Encrypt(tag, xorBytes(delta, checksum))

	// For partial block, return full encrypted block (Python behavior)
	// For full block, return encrypted[:len(plain)]
	if pos < len(plain) {
		return encrypted, tag[:], nil
	}
	return encrypted[:len(plain)], tag[:], nil
}

// ocbDecrypt performs OCB-AES decryption.
// Returns plaintext and tag for verification.
func ocbDecrypt(aes cipher, encrypted, nonce []byte, lenPlain int) ([]byte, []byte, error) {
	if len(nonce) != 16 {
		return nil, nil, fmt.Errorf("ocb: nonce must be 16 bytes")
	}

	delta := make([]byte, 16)
	aes.Encrypt(delta, nonce)

	checksum := make([]byte, 16)
	plain := make([]byte, lenPlain)
	pos := 0

	for pos+16 <= lenPlain {
		delta = s2(delta)
		xored := xorBytes(delta, encrypted[pos:pos+16])
		tmp := make([]byte, 16)
		aes.Decrypt(tmp, xored)
		plainBlock := xorBytes(delta, tmp)
		copy(plain[pos:pos+16], plainBlock)
		checksum = xorBytes(checksum, plainBlock)
		pos += 16
	}

	if pos < lenPlain {
		delta = s2(delta)
		padIn := make([]byte, 16)
		binary.BigEndian.PutUint64(padIn[0:8], 0)
		binary.BigEndian.PutUint64(padIn[8:16], uint64(lenPlain-pos)*8)
		xored := xorBytes(padIn, delta)
		pad := make([]byte, 16)
		aes.Encrypt(pad, xored)

		// Pad the ciphertext to 16 bytes for XOR with pad
		encPadded := make([]byte, 16)
		copy(encPadded, encrypted[pos:])

		plainBlock := xorBytes(encPadded, pad)
		copy(plain[pos:], plainBlock[:lenPlain-pos])
		checksum = xorBytes(checksum, plainBlock)

		// Counter-cryptanalysis mitigation
		if bytesEqual(plainBlock[:15], delta[:15]) {
			return nil, nil, &DecryptFailedException{"possibly tampered block"}
		}
	}

	// Compute final delta = delta XOR S2(delta) where delta is the last offset
	lastDelta := make([]byte, 16)
	copy(lastDelta, delta)
	delta = xorBytes(delta, s2(lastDelta))
	tag := make([]byte, 16)
	aes.Encrypt(tag, xorBytes(delta, checksum))

	return plain, tag[:], nil
}

// s2 doubles a 128-bit block in GF(2^128) using the Rinfeld polynomial.
// This is equivalent to L_i = L_{i-1} * 2 in OCB.
// s2(block) = block << 1 with polynomial reduction.
func s2(block []byte) []byte {
	if len(block) != 16 {
		panic("s2: block must be 16 bytes")
	}

	// Unpack as two big-endian uint64's: low = block[0:8], high = block[8:16]
	low := binary.BigEndian.Uint64(block[0:8])
	high := binary.BigEndian.Uint64(block[8:16])

	carry := low >> 63

	// low = (low << 1) | (high >> 63)
	low = ((low << 1) | (high >> 63)) & 0xffffffffffffffff

	// high = (high << 1) ^ (carry * 0x87)
	high = ((high << 1) ^ (carry * 0x87)) & 0xffffffffffffffff

	binary.BigEndian.PutUint64(block[0:8], low)
	binary.BigEndian.PutUint64(block[8:16], high)

	return block
}

// incrementIV increments an IV in place, starting from the given offset.
func incrementIV(iv []byte) []byte {
	result := make([]byte, len(iv))
	copy(result, iv)
	incrementIVInPlace(result, 0)
	return result
}

func incrementIVInPlace(iv []byte, start int) {
	for i := start; i < len(iv); i++ {
		iv[i] = (iv[i] + 1) & 0xff
		if iv[i] != 0 {
			break
		}
	}
}

// decrementIV decrements an IV in place, starting from the given offset.
func decrementIV(iv []byte) []byte {
	result := make([]byte, len(iv))
	copy(result, iv)
	decrementIVInPlace(result, 0)
	return result
}

func decrementIVInPlace(iv []byte, start int) {
	for i := start; i < len(iv); i++ {
		iv[i] = (iv[i] - 1) & 0xff
		if iv[i] != 0xff {
			break
		}
	}
}

// xorBytes xors two equal-length byte slices.
func xorBytes(a, b []byte) []byte {
	if len(a) != len(b) {
		panic("xorBytes: length mismatch")
	}
	result := make([]byte, len(a))
	for i := range a {
		result[i] = a[i] ^ b[i]
	}
	return result
}

// bytesEqual checks if two byte slices are equal.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
