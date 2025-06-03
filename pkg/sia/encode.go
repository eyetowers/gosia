package sia

import (
	"fmt"
	"strings"
)

func Encode(sequence uint16, i Identity, m Message) string {
	payload := fmt.Sprintf("\"%s\"%04dL0%s", m.ID(), sequence, toBody(i, m))
	length := len(payload)
	crc := checksum([]byte(payload))
	return fmt.Sprintf("\n%04X%04X%s\r", crc, length, payload)
}

func toBody(i Identity, m Message) string {
	var result strings.Builder
	result.WriteRune('#')
	result.WriteString(i.AuthCode)
	result.WriteRune('[')
	result.WriteString(m.Payload(i.AuthCode))
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
	out.WriteString(ts.UTC().Format("_15:04:05,01-02-2006"))
}
