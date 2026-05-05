package sia

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var lineRE = regexp.MustCompile(`^[[:xdigit:]]{1,6}$`)

const timestampLayout = "_15:04:05,01-02-2006"

// Encode converts a message into a complete SIA DC-09 frame.
func Encode(sequence uint16, i Identity, m Message) (string, error) {
	line, err := linePrefix(i)
	if err != nil {
		return "", err
	}
	key := i.key()
	if err := validateAESKey(key); err != nil {
		return "", err
	}

	id := m.ID()
	body := toBody(i, m)
	if len(key) > 0 {
		id = encryptedID(id)
		if m.Timestamp().IsZero() {
			body += time.Now().UTC().Format(timestampLayout)
		}
	}

	payload := fmt.Sprintf("\"%s\"%04dL%s%s", id, sequence, line, body)
	payload, err = encryptPayload(payload, key)
	if err != nil {
		return "", err
	}
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

func appendTimestamp(out *strings.Builder, m Message) {
	ts := m.Timestamp()
	if ts.IsZero() {
		return
	}
	out.WriteString(ts.UTC().Format(timestampLayout))
}
