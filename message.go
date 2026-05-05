package sia

import "time"

// Message is a SIA DC-09 message payload that can be encoded into a frame.
type Message interface {
	Payload(account string) string
	ID() string
	Metadata() map[Metadata]string
	Timestamp() time.Time
}
