package sia

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// zeroIV is the all-zero initialization vector mandated by ANSI/SIA DC-09-2013 p.16.
var zeroIV = make([]byte, aes.BlockSize)

// encryptPayload encrypts plaintext for a DC-09 encrypted frame.
// A random pad and a '|' separator are prepended so the total length is a
// multiple of the AES block size (minimum one byte of padding). The combined
// buffer is encrypted with AES-CBC using the zero IV defined by the spec.
// Returns uppercase hex ciphertext.
func encryptPayload(key, plaintext []byte) (string, error) {
	padLen := aes.BlockSize - (1+len(plaintext))%aes.BlockSize
	if padLen == 0 {
		padLen = aes.BlockSize
	}

	pad, err := randomPadding(padLen)
	if err != nil {
		return "", err
	}

	// Buffer layout: <pad> '|' <plaintext>
	clear := make([]byte, padLen+1+len(plaintext))
	copy(clear, pad)
	clear[padLen] = '|'
	copy(clear[padLen+1:], plaintext)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating AES cipher: %w", err)
	}
	ct := make([]byte, len(clear))
	cipher.NewCBCEncrypter(block, zeroIV).CryptBlocks(ct, clear)
	return strings.ToUpper(hex.EncodeToString(ct)), nil
}

// decryptPayload hex-decodes and AES-CBC-decrypts (zero IV) the ciphertext,
// then strips the random padding that precedes the first '|' delimiter.
func decryptPayload(key []byte, hexCiphertext string) ([]byte, error) {
	ct, err := hex.DecodeString(hexCiphertext)
	if err != nil {
		return nil, fmt.Errorf("decoding hex: %w", err)
	}
	if len(ct) == 0 || len(ct)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext length %d is not a non-zero multiple of block size", len(ct))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating AES cipher: %w", err)
	}
	plain := make([]byte, len(ct))
	cipher.NewCBCDecrypter(block, zeroIV).CryptBlocks(plain, ct)

	idx := bytes.IndexByte(plain, '|')
	if idx < 0 {
		return nil, fmt.Errorf("no '|' delimiter in decrypted content")
	}
	return plain[idx+1:], nil
}

// randomPadding returns n random bytes with '[', ']', and '|' excluded (they
// are DC-09 frame delimiters and must not appear in the padding region).
func randomPadding(n int) ([]byte, error) {
	pad := make([]byte, 0, n)
	buf := make([]byte, n+n/2+4) // overallocate to avoid multiple read calls
	for len(pad) < n {
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generating padding: %w", err)
		}
		for _, b := range buf {
			if b != '[' && b != ']' && b != '|' {
				pad = append(pad, b)
				if len(pad) == n {
					break
				}
			}
		}
	}
	return pad, nil
}
