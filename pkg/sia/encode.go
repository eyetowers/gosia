package sia

import (
	"fmt"
	"regexp"
	"strings"
)

var lineRE = regexp.MustCompile(`^[[:xdigit:]]{1,6}$`)

func Encode(sequence uint16, i Identity, m Message) (string, error) {
	line, err := linePrefix(i)
	if err != nil {
		return "", err
	}
	payload := fmt.Sprintf("\"%s\"%04dL%s%s", m.ID(), sequence, line, toBody(i, m))
	length := len(payload)
	crc := checksum([]byte(payload))
	return fmt.Sprintf("\n%04X%04X%s\r", crc, length, payload), nil
}

func linePrefix(i Identity) (string, error) {
	if i.Line == "" {
		return "0", nil
	}
	if !lineRE.MatchString(i.Line) {
		return "", fmt.Errorf("invalid SIA line %q: must be 1-6 hex digits", i.Line)
	}
	return i.Line, nil
}

func toBody(i Identity, m Message) string {
	var result strings.Builder
	result.WriteRune('#')
	result.WriteString(i.Account)
	result.WriteRune('[')
	result.WriteString(m.Payload(i.Account))
	result.WriteRune(']')
	appendMetadata(&result, m)
	appendTimestamp(&result, m)
	return result.String()
}

func appendMetadata(out *strings.Builder, m Message) {
	data := m.Metadata()
	if len(data) == 0 {
		return
	}
	for k, v := range data {
		out.WriteRune('[')
		out.WriteString(string(k))
		out.WriteString(v)
		out.WriteRune(']')
	}
}

// EncodeEncrypted encodes an encrypted DC-09 frame per ANSI/SIA DC-09-2013 p.16.
// The data section (payload + closing ']' + metadata + timestamp) is encrypted
// with AES-CBC (zero IV) using key. The message ID gains a '*' prefix in the
// frame to signal encryption. NAK and DUH must never be encrypted.
func EncodeEncrypted(sequence uint16, i Identity, m Message, key []byte) (string, error) {
	if id := m.ID(); id == "NAK" || id == "DUH" {
		return "", fmt.Errorf("DC-09: %q must not be encrypted", id)
	}
	line, err := linePrefix(i)
	if err != nil {
		return "", err
	}

	// Build the content to encrypt: everything that normally sits between '[' and
	// end-of-body — the payload, the closing ']', optional metadata, optional timestamp.
	var inner strings.Builder
	inner.WriteString(m.Payload(i.Account))
	inner.WriteRune(']')
	appendMetadata(&inner, m)
	appendTimestamp(&inner, m)

	ciphertext, err := encryptPayload(key, []byte(inner.String()))
	if err != nil {
		return "", fmt.Errorf("encrypting SIA DC-09 payload: %w", err)
	}

	// Encrypted frame: "*id"seq address "#" account "[" ciphertext (no closing ']')
	payload := fmt.Sprintf("\"*%s\"%04dL%s#%s[%s", m.ID(), sequence, line, i.Account, ciphertext)
	crc := checksum([]byte(payload))
	return fmt.Sprintf("\n%04X%04X%s\r", crc, len(payload), payload), nil
}

func appendTimestamp(out *strings.Builder, m Message) {
	ts := m.Timestamp()
	if ts.IsZero() {
		return
	}
	out.WriteString(ts.UTC().Format("_15:04:05,01-02-2006"))
}
