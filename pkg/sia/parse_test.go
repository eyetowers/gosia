package sia

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// frame builds a syntactically valid DC-09 frame with the correct CRC and
// length fields for the given payload, so test cases can focus on the body.
func frame(payload string) string {
	crc := checksum([]byte(payload))
	return fmt.Sprintf("\n%04X%04X%s\r", crc, len(payload), payload)
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
