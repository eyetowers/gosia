package sia

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// frame builds a syntactically valid DC-09 frame with the correct CRC and
// length fields for the given payload, so test cases can focus on the body.
func frame(payload string) string {
	crc := checksum([]byte(payload))
	return fmt.Sprintf("\n%04X%04X%s\r", crc, len(payload), payload)
}

func encryptedFrame(payload string, key []byte) string {
	encrypted, err := encryptPayload(payload, key)
	if err != nil {
		panic(err)
	}
	return frame(encrypted)
}

func TestParse_ValidFrames(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    ParsedFrame
	}{
		{
			name:    "ACK without receiver",
			payload: `"ACK"0042L0#ABCD[]`,
			want: ParsedFrame{
				Message:  empty{id: "ACK"},
				Sequence: 42,
				Line:     "0",
				Account:  "ABCD",
			},
		},
		{
			name:    "ACK with R receiver",
			payload: `"ACK"0042R7L0#ABCD[]`,
			want: ParsedFrame{
				Message:  empty{id: "ACK"},
				Sequence: 42,
				Receiver: "7",
				Line:     "0",
				Account:  "ABCD",
			},
		},
		{
			name:    "single-digit account",
			payload: `"ACK"0001L1#F[]`,
			want: ParsedFrame{
				Message:  empty{id: "ACK"},
				Sequence: 1,
				Line:     "1",
				Account:  "F",
			},
		},
		{
			name:    "six-digit account",
			payload: `"ACK"0001L1#ABCDEF[]`,
			want: ParsedFrame{
				Message:  empty{id: "ACK"},
				Sequence: 1,
				Line:     "1",
				Account:  "ABCDEF",
			},
		},
		{
			name:    "four-digit line",
			payload: `"ACK"0001L1A2B#ABCD[]`,
			want: ParsedFrame{
				Message:  empty{id: "ACK"},
				Sequence: 1,
				Line:     "1A2B",
				Account:  "ABCD",
			},
		},
		{
			name:    "six-digit line",
			payload: `"ACK"0001L1A2B3C#ABCD[]`,
			want: ParsedFrame{
				Message:  empty{id: "ACK"},
				Sequence: 1,
				Line:     "1A2B3C",
				Account:  "ABCD",
			},
		},
		{
			name:    "with timestamp",
			payload: `"ACK"0099L0#ABCD[]_12:34:56,01-02-2026`,
			want: ParsedFrame{
				Message:  empty{id: "ACK"},
				Sequence: 99,
				Line:     "0",
				Account:  "ABCD",
			},
		},
		{
			name:    "NAK with L0A0 parses accountless line",
			payload: `"NAK"0000R0L0A0[]`,
			want: ParsedFrame{
				Message:  empty{id: "NAK"},
				Sequence: 0,
				Receiver: "0",
				Line:     "0A0",
			},
		},
		{
			name:    "multiple data blocks (data plus metadata)",
			payload: `"ADM-CID"0007L0#1234[abc][V1]`,
			want: ParsedFrame{
				Message:  empty{id: "ADM-CID"},
				Sequence: 7,
				Line:     "0",
				Account:  "1234",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(frame(tc.payload))
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", tc.payload, err)
			}
			if got != tc.want {
				t.Errorf("Parse(%q) = %#v, want %#v", tc.payload, got, tc.want)
			}
		})
	}
}

func TestParse_ErrorTypes(t *testing.T) {
	good := frame(`"ACK"0001L0#ABCD[]`)

	cases := []struct {
		name string
		msg  string
		want error
	}{
		{
			name: "missing leading LF",
			msg:  strings.TrimPrefix(good, "\n"),
			want: ErrMalformedFrame,
		},
		{
			name: "missing trailing CR",
			msg:  strings.TrimSuffix(good, "\r"),
			want: ErrMalformedFrame,
		},
		{
			name: "truncated frame (no payload)",
			msg:  "\n0000\r",
			want: ErrMalformedFrame,
		},
		{
			name: "non-hex CRC",
			msg:  "\nZZZZ0001\"ACK\"0001L0#ABCD[]\r",
			want: ErrMalformedFrame,
		},
		{
			name: "CRC mismatch",
			msg: func() string {
				p := `"ACK"0001L0#ABCD[]`
				return fmt.Sprintf("\n0000%04X%s\r", len(p), p)
			}(),
			want: ErrCRCMismatch,
		},
		{
			name: "length mismatch",
			msg: func() string {
				p := `"ACK"0001L0#ABCD[]`
				return fmt.Sprintf("\n%04X%04X%s\r", checksum([]byte(p)), len(p)+1, p)
			}(),
			want: ErrLengthMismatch,
		},
		{
			name: "missing L line",
			msg:  frame(`"ACK"0001#ABCD[]`),
			want: ErrMalformedFrame,
		},
		{
			name: "unterminated data block",
			msg:  frame(`"ACK"0001L0#ABCD[`),
			want: ErrMalformedFrame,
		},
		{
			name: "bad timestamp",
			msg:  frame(`"ACK"0001L0#ABCD[]_not-a-timestamp`),
			want: ErrMalformedFrame,
		},
		{
			name: "trailing junk",
			msg:  frame(`"ACK"0001L0#ABCD[]xyz`),
			want: ErrMalformedFrame,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.msg)
			if !errors.Is(err, tc.want) {
				t.Errorf("Parse(%q) = %v, want errors.Is(_, %v)", tc.msg, err, tc.want)
			}
		})
	}
}

func TestParse_RoundTripFromEncoder(t *testing.T) {
	identity := Identity{Account: "ABCD"}
	encoded, err := Encode(123, identity, Ack)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	got, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse(Encode(...)) error: %v", err)
	}
	if got.Message.ID() != Ack.ID() {
		t.Errorf("Message.ID() = %q, want %q", got.Message.ID(), Ack.ID())
	}
	if got.Sequence != 123 {
		t.Errorf("Sequence = %d, want 123", got.Sequence)
	}
	if got.Line != "0" {
		t.Errorf("Line = %q, want %q", got.Line, "0")
	}
	if got.Account != identity.Account {
		t.Errorf("Account = %q, want %q", got.Account, identity.Account)
	}
}

func TestEncode_LinePrefix(t *testing.T) {
	encoded, err := Encode(123, Identity{Account: "ABCD", Line: "1A2B"}, Ack)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	got, err := Parse(encoded)
	if err != nil {
		t.Fatalf("Parse(Encode(...)) error: %v", err)
	}
	if got.Line != "1A2B" {
		t.Errorf("Line = %q, want %q", got.Line, "1A2B")
	}
}

func TestEncode_InvalidLinePrefix(t *testing.T) {
	_, err := Encode(123, Identity{Account: "ABCD", Line: "1234567"}, Ack)
	if err == nil {
		t.Fatal("Encode returned nil error for invalid line")
	}
}

func TestEncodeEncrypted_RoundTrip(t *testing.T) {
	key := []byte("0123456789ABCDEF")
	identity := Identity{Account: "ABCD"}.WithEncryptionKey(key)
	message := DCS(
		"RP",
		Timestamp(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
	)

	encoded, err := Encode(123, identity, message)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	payload, err := unframe(encoded)
	if err != nil {
		t.Fatalf("unframe returned error: %v", err)
	}
	if !strings.Contains(payload, `"*SIA-DCS"0123L0#ABCD[`) {
		t.Fatalf("encrypted payload = %q, want encrypted SIA-DCS header", payload)
	}
	if strings.Contains(payload, "|NRP") {
		t.Fatalf("encrypted payload leaked DCS data: %q", payload)
	}

	_, err = Parse(encoded)
	if !errors.Is(err, ErrEncryptedMessage) {
		t.Fatalf("Parse encrypted without key = %v, want ErrEncryptedMessage", err)
	}

	got, err := ParseWithKey(encoded, key)
	if err != nil {
		t.Fatalf("ParseWithKey returned error: %v", err)
	}
	want := ParsedFrame{
		Message:   empty{id: "SIA-DCS"},
		Sequence:  123,
		Line:      "0",
		Account:   "ABCD",
		Encrypted: true,
	}
	if got != want {
		t.Errorf("ParseWithKey = %#v, want %#v", got, want)
	}
}

func TestEncodeEncrypted_ACKAddsTimestamp(t *testing.T) {
	key := []byte("0123456789ABCDEF01234567")
	encoded, err := Encode(7, Identity{Account: "ABCD"}.WithEncryptionKey(key), Ack)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	got, err := ParseWithKey(encoded, key)
	if err != nil {
		t.Fatalf("ParseWithKey returned error: %v", err)
	}
	if got.Message.ID() != Ack.ID() || !got.Encrypted {
		t.Fatalf("ParseWithKey = %#v, want encrypted ACK", got)
	}
}

func TestEncodeEncrypted_AES256RoundTrip(t *testing.T) {
	key := []byte("0123456789ABCDEF0123456789ABCDEF")
	identity := Identity{Account: "CAFE", Line: "1A"}.WithEncryptionKey(key)
	message := DCS(
		"BA",
		Zone(2, "Zone 2"),
		Area(1, "Partition 1"),
		Timestamp(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)),
	)

	encoded, err := Encode(9999, identity, message)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	got, err := ParseWithKey(encoded, key)
	if err != nil {
		t.Fatalf("ParseWithKey returned error: %v", err)
	}
	want := ParsedFrame{
		Message:   empty{id: "SIA-DCS"},
		Sequence:  9999,
		Line:      "1A",
		Account:   "CAFE",
		Encrypted: true,
	}
	if got != want {
		t.Fatalf("ParseWithKey = %#v, want %#v", got, want)
	}
}

func TestParseEncrypted_ACKWithoutPadTerminator(t *testing.T) {
	key, err := ParseEncryptionKey("0123456789ABCDEF0123456789ABCDEF")
	if err != nil {
		t.Fatalf("ParseEncryptionKey returned error: %v", err)
	}
	msg := "\nC1B30052\"*ACK\"0001L0#1001[FFAC5F1A3794837779D9C95762764263A68961C08ABD2942B4607F2321BF554A\r"

	got, err := ParseWithKey(msg, key)
	if err != nil {
		t.Fatalf("ParseWithKey returned error: %v", err)
	}
	want := ParsedFrame{
		Message:   empty{id: "ACK"},
		Sequence:  1,
		Line:      "0",
		Account:   "1001",
		Encrypted: true,
	}
	if got != want {
		t.Fatalf("ParseWithKey = %#v, want %#v", got, want)
	}
}

func TestParseEncrypted_WrongValidKeyFails(t *testing.T) {
	key := []byte("0123456789ABCDEF")
	wrongKey := []byte("FEDCBA9876543210")
	encoded := encryptedFrame(
		`"*ACK"0001L0#ABCD[|]_03:04:05,01-02-2026`,
		key,
	)

	_, err := ParseWithKey(encoded, wrongKey)
	if err == nil {
		t.Fatal("ParseWithKey returned nil error for wrong AES key")
	}
	if !errors.Is(err, ErrEncryption) && !errors.Is(err, ErrMalformedFrame) {
		t.Fatalf("ParseWithKey wrong key error = %v, want ErrEncryption or ErrMalformedFrame", err)
	}
}

func TestParseEncrypted_MalformedEncryptedRegions(t *testing.T) {
	key := []byte("0123456789ABCDEF")
	cases := []struct {
		name string
		msg  string
		want error
	}{
		{
			name: "non-hex ciphertext",
			msg:  frame(`"*ACK"0001L0#ABCD[not-hex]`),
			want: ErrEncryption,
		},
		{
			name: "ciphertext not block aligned",
			msg:  frame(`"*ACK"0001L0#ABCD[001122]`),
			want: ErrEncryption,
		},
		{
			name: "missing data block",
			msg:  frame(`"*ACK"0001L0#ABCD`),
			want: ErrMalformedFrame,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseWithKey(tc.msg, key)
			if !errors.Is(err, tc.want) {
				t.Fatalf("ParseWithKey = %v, want errors.Is(_, %v)", err, tc.want)
			}
		})
	}
}

func TestEncryption_InvalidKeyLength(t *testing.T) {
	key := []byte("too short")
	_, err := Encode(1, Identity{Account: "ABCD"}.WithEncryptionKey(key), Ack)
	if !errors.Is(err, ErrEncryption) {
		t.Fatalf("Encode with invalid key = %v, want ErrEncryption", err)
	}

	encrypted, err := Encode(1, Identity{Account: "ABCD"}.WithEncryptionKey([]byte("0123456789ABCDEF")), Ack)
	if err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	_, err = ParseWithKey(encrypted, key)
	if !errors.Is(err, ErrEncryption) {
		t.Fatalf("ParseWithKey with invalid key = %v, want ErrEncryption", err)
	}
}

func TestParseEncryptionKey(t *testing.T) {
	key, err := ParseEncryptionKey("30313233343536373839414243444546")
	if err != nil {
		t.Fatalf("ParseEncryptionKey returned error: %v", err)
	}
	if string(key) != "0123456789ABCDEF" {
		t.Fatalf("ParseEncryptionKey = %q, want raw AES key bytes", string(key))
	}

	_, err = ParseEncryptionKey("ABCD")
	if !errors.Is(err, ErrEncryption) {
		t.Fatalf("ParseEncryptionKey short key = %v, want ErrEncryption", err)
	}
}

func TestClassifyResponse_EncryptedACK(t *testing.T) {
	key := []byte("0123456789ABCDEF")
	identity := Identity{Account: "ABCD"}.WithEncryptionKey(key)
	encoded := encryptedFrame(
		`"*ACK"0042L0#ABCD[|]_03:04:05,01-02-2026`,
		key,
	)

	parsed, err := ParseWithKey(encoded, key)
	if err != nil {
		t.Fatalf("ParseWithKey returned error: %v", err)
	}
	if err := classifyResponse(parsed, 42, identity); err != nil {
		t.Fatalf("classifyResponse returned error: %v", err)
	}
}

func TestClassifyResponse(t *testing.T) {
	anyErr := errors.New("any error")

	cases := []struct {
		name     string
		payload  string
		sequence uint16
		identity Identity
		want     error
		contains string
	}{
		{
			name:     "ACK is success",
			payload:  `"ACK"0000L0#ABCD[]`,
			identity: Identity{Account: "ABCD"},
		},
		{
			name:     "ACK mismatched account fails",
			payload:  `"ACK"0000L0#CAFE[]`,
			identity: Identity{Account: "ABCD"},
			want:     anyErr,
			contains: "mismatched account",
		},
		{
			name:     "NAK becomes *NakError despite mismatched accountless line",
			payload:  `"NAK"0000R0L0A0[]`,
			identity: Identity{Account: "ABCD", Line: "0"},
			want:     &NakError{},
		},
		{
			name:     "DUH becomes *DuhError despite mismatched account",
			payload:  `"DUH"0000L1#FFFF[]`,
			identity: Identity{Account: "ABCD", Line: "0"},
			want:     &DuhError{},
		},
		{
			name:     "sequence mismatch fails before response type",
			payload:  `"NAK"0001L0#ABCD[]`,
			sequence: 2,
			identity: Identity{Account: "ABCD"},
			want:     anyErr,
			contains: "mismatched sequence",
		},
		{
			name:     "unknown id is generic error",
			payload:  `"WAT"0000L0#ABCD[]`,
			identity: Identity{Account: "ABCD"},
			want:     anyErr,
			contains: "unexpected reply",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parsed, err := Parse(frame(tc.payload))
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			err = classifyResponse(parsed, tc.sequence, tc.identity)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("classifyResponse returned error: %v", err)
				}
				return
			}
			if tc.want != anyErr && !errors.Is(err, tc.want) {
				t.Fatalf("classifyResponse error = %T %v, want errors.Is(_, %T)", err, err, tc.want)
			}
			if tc.want == anyErr && err == nil {
				t.Fatalf("classifyResponse returned nil error")
			}
			if tc.contains != "" && !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("classifyResponse error = %q, want substring %q", err, tc.contains)
			}
		})
	}
}
