package sia

import (
	"fmt"
)

func Encode(sequence uint16, i Identity, m Message) string {
	payload := fmt.Sprintf("\"%s\"%04dL0%s", m.ID(), sequence, toBody(i, m))
	length := len(payload)
	crc := checksum([]byte(payload))
	return fmt.Sprintf("\n%04X%04X%s\r", crc, length, payload)
}

func toBody(i Identity, m Message) string {
	return fmt.Sprintf("#%s[%s]", i.AuthCode, m.Payload(i.AuthCode))
}
