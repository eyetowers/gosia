package sia

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var testKey128 = []byte("0123456789ABCDEF") // 16 bytes / AES-128

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cases := []string{
		"]",
		"]_12:34:56,01-02-2026",
		"#ABCD|NRP0000]",
		"#ABCD|NRP0000]_12:34:56,01-02-2026",
	}
	for _, plain := range cases {
		t.Run(plain, func(t *testing.T) {
			hex, err := encryptPayload(testKey128, []byte(plain))
			if err != nil {
				t.Fatalf("encryptPayload: %v", err)
			}
			got, err := decryptPayload(testKey128, hex)
			if err != nil {
				t.Fatalf("decryptPayload: %v", err)
			}
			if string(got) != plain {
				t.Errorf("round-trip: got %q, want %q", got, plain)
			}
		})
	}
}

func TestEncryptedFrameRoundTrip(t *testing.T) {
	identity := Identity{Account: "ABCD"}
	keys := MapKeyStore{"ABCD": testKey128}

	cases := []struct {
		name string
		msg  Message
	}{
		{"NULL", Null},
		{"ACK", Ack},
		{"SIA-DCS", DCS("RP", Timestamp(time.Date(2026, 1, 2, 12, 34, 56, 0, time.UTC)))},
		{"SIA-DCS with zone", DCS("BA", Zone(3, "Front door"))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeEncrypted(42, identity, tc.msg, testKey128)
			if err != nil {
				t.Fatalf("EncodeEncrypted: %v", err)
			}

			// Must contain the encrypted ID marker.
			if !strings.Contains(encoded, "\"*") {
				t.Errorf("encoded frame missing '*' prefix: %q", encoded)
			}

			got, err := ParseEncrypted(encoded, keys)
			if err != nil {
				t.Fatalf("ParseEncrypted: %v", err)
			}
			if got.Message.ID() != tc.msg.ID() {
				t.Errorf("ID = %q, want %q", got.Message.ID(), tc.msg.ID())
			}
			if got.Sequence != 42 {
				t.Errorf("Sequence = %d, want 42", got.Sequence)
			}
			if got.Account != identity.Account {
				t.Errorf("Account = %q, want %q", got.Account, identity.Account)
			}
			if !got.Encrypted {
				t.Error("Encrypted = false, want true")
			}
		})
	}
}

func TestEncryptedFramePlainFallback(t *testing.T) {
	// ParseEncrypted should transparently handle plain frames.
	identity := Identity{Account: "ABCD"}
	encoded, err := Encode(7, identity, Ack)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got, err := ParseEncrypted(encoded, MapKeyStore{})
	if err != nil {
		t.Fatalf("ParseEncrypted on plain frame: %v", err)
	}
	if got.Encrypted {
		t.Error("Encrypted = true for plain frame")
	}
	if got.Message.ID() != "ACK" {
		t.Errorf("ID = %q, want ACK", got.Message.ID())
	}
}

func TestEncryptedNoKey(t *testing.T) {
	encoded, err := EncodeEncrypted(1, Identity{Account: "ABCD"}, Null, testKey128)
	if err != nil {
		t.Fatalf("EncodeEncrypted: %v", err)
	}
	_, err = ParseEncrypted(encoded, MapKeyStore{}) // empty key store
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Errorf("error = %v, want ErrDecryptionFailed", err)
	}
}

func TestEncryptNakDuhRejected(t *testing.T) {
	for _, msg := range []Message{Nak, Duh} {
		_, err := EncodeEncrypted(1, Identity{Account: "ABCD"}, msg, testKey128)
		if err == nil {
			t.Errorf("EncodeEncrypted(%s) returned nil error, want error", msg.ID())
		}
	}
}

func TestEncryptedRoundTripDifferentKeySizes(t *testing.T) {
	keys := [][]byte{
		[]byte("0123456789ABCDEF"),         // AES-128
		[]byte("0123456789ABCDEF01234567"), // AES-192
		[]byte("0123456789ABCDEF0123456789ABCDEF"), // AES-256
	}
	identity := Identity{Account: "FF01"}
	for _, key := range keys {
		ks := MapKeyStore{"FF01": key}
		encoded, err := EncodeEncrypted(1, identity, Null, key)
		if err != nil {
			t.Fatalf("EncodeEncrypted (keylen=%d): %v", len(key), err)
		}
		got, err := ParseEncrypted(encoded, ks)
		if err != nil {
			t.Fatalf("ParseEncrypted (keylen=%d): %v", len(key), err)
		}
		if !got.Encrypted {
			t.Errorf("keylen=%d: Encrypted = false", len(key))
		}
	}
}
