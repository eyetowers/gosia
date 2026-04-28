package sia

import "time"

type Message interface {
	Payload(account string) string
	ID() string
	Metadata() map[Metadata]string
	Timestamp() time.Time
}
