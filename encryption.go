package sia

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

const encryptedIDPrefix = "*"

var (
	// ErrEncryptedMessage reports that a frame is encrypted but no key was supplied.
	ErrEncryptedMessage = errors.New("encrypted SIA DC-09 message")
	// ErrEncryption reports AES key, encrypted payload, or encryption processing failures.
	ErrEncryption = errors.New("SIA DC-09 encryption error")
)

// ParseEncryptionKey decodes and validates a hex-encoded AES key.
func ParseEncryptionKey(input string) ([]byte, error) {
	key, err := hex.DecodeString(strings.TrimSpace(input))
	if err != nil {
		return nil, fmt.Errorf("%w: AES key must be hex: %v", ErrEncryption, err)
	}
	if err := validateAESKey(key); err != nil {
		return nil, err
	}
	if len(key) == 0 {
		return nil, fmt.Errorf("%w: AES key must not be empty", ErrEncryption)
	}
	return key, nil
}

func validateAESKey(key []byte) error {
	switch len(key) {
	case 0, 16, 24, 32:
		return nil
	default:
		return fmt.Errorf("%w: AES key must be 16, 24, or 32 bytes, got %d", ErrEncryption, len(key))
	}
}

func encryptedID(id string) string {
	if strings.HasPrefix(id, encryptedIDPrefix) {
		return id
	}
	return encryptedIDPrefix + id
}

func isEncryptedID(id string) bool {
	return strings.HasPrefix(id, encryptedIDPrefix)
}

func plaintextID(id string) string {
	return strings.TrimPrefix(id, encryptedIDPrefix)
}

func encryptPayload(payload string, key []byte) (string, error) {
	if err := validateAESKey(key); err != nil {
		return "", err
	}
	if len(key) == 0 {
		return payload, nil
	}

	prefix, plaintext, err := splitEncryptedRegion(payload)
	if err != nil {
		return "", err
	}

	padded, err := addEncryptionPad(plaintext)
	if err != nil {
		return "", err
	}
	ciphertext, err := aesCBC(key, padded, cipher.NewCBCEncrypter)
	if err != nil {
		return "", err
	}
	return prefix + strings.ToUpper(hex.EncodeToString(ciphertext)), nil
}

func decryptPayload(payload string, key []byte) (string, error) {
	if err := validateAESKey(key); err != nil {
		return "", err
	}
	if len(key) == 0 {
		return "", ErrEncryptedMessage
	}

	prefix, encoded, err := splitEncryptedRegion(payload)
	if err != nil {
		return "", err
	}
	ciphertext, err := decodeEncryptedRegion(encoded)
	if err != nil {
		return "", err
	}
	plaintext, err := aesCBC(key, ciphertext, cipher.NewCBCDecrypter)
	if err != nil {
		return "", err
	}

	body, err := removeEncryptionPad(plaintext)
	if err != nil {
		return "", err
	}
	return unmarkEncryptedPrefix(prefix) + string(body), nil
}

func splitEncryptedRegion(payload string) (string, []byte, error) {
	open := strings.IndexByte(payload, '[')
	if open < 0 {
		return "", nil, fmt.Errorf("%w: encrypted payload has no data block", ErrMalformedFrame)
	}
	return payload[:open+1], []byte(payload[open+1:]), nil
}

func addEncryptionPad(region []byte) ([]byte, error) {
	padLen := aes.BlockSize - ((len(region) + 1) % aes.BlockSize)
	if padLen == 0 {
		padLen = aes.BlockSize
	}

	pad, err := randomPad(padLen)
	if err != nil {
		return nil, err
	}

	padded := make([]byte, 0, len(pad)+1+len(region))
	padded = append(padded, pad...)
	padded = append(padded, '|')
	padded = append(padded, region...)
	return padded, nil
}

func removeEncryptionPad(region []byte) ([]byte, error) {
	if sep := bytes.IndexByte(region, '|'); sep >= 0 {
		return region[sep+1:], nil
	}

	// Some receivers return empty encrypted ACKs without the spec-defined
	// pad terminator. In that case the plaintext begins at the closing data
	// bracket after random padding.
	if sep := bytes.IndexByte(region, ']'); sep >= 0 {
		return region[sep:], nil
	}
	return nil, fmt.Errorf("%w: encrypted region has no pad terminator", ErrEncryption)
}

func decodeEncryptedRegion(encoded []byte) ([]byte, error) {
	ciphertext := make([]byte, hex.DecodedLen(len(encoded)))
	n, err := hex.Decode(ciphertext, encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: encrypted region is not ASCII hex: %v", ErrEncryption, err)
	}
	ciphertext = ciphertext[:n]
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: encrypted region length %d is not a positive AES block multiple", ErrEncryption, len(ciphertext))
	}
	return ciphertext, nil
}

type blockMode func(cipher.Block, []byte) cipher.BlockMode

func aesCBC(key, input []byte, mode blockMode) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryption, err)
	}
	output := append([]byte(nil), input...)
	mode(block, make([]byte, aes.BlockSize)).CryptBlocks(output, output)
	return output, nil
}

func unmarkEncryptedPrefix(prefix string) string {
	return strings.Replace(prefix, `"`+encryptedIDPrefix, `"`, 1)
}

func randomPad(n int) ([]byte, error) {
	out := make([]byte, n)
	for i := range out {
		b, err := randomPadByte()
		if err != nil {
			return nil, err
		}
		out[i] = b
	}
	return out, nil
}

func randomPadByte() (byte, error) {
	var b [1]byte
	for {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, fmt.Errorf("%w: generating pad: %v", ErrEncryption, err)
		}
		if b[0] != '|' && b[0] != '[' && b[0] != ']' {
			return b[0], nil
		}
	}
}
