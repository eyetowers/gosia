package sia

import "time"

type Message interface {
	Payload(authCode string) string
	ID() string
	Metadata() map[Metadata]string
	Timestamp() time.Time
}
