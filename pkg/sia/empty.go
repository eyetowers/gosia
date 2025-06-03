package sia

import "time"

var (
	Null = empty{id: "NULL"}
	Ack  = empty{id: "ACK"}
	Nack = empty{id: "NACK"}
	Duh  = empty{id: "DUH"}
)

type empty struct {
	id string
}

func (m empty) ID() string {
	return m.id
}

func (m empty) Payload(_ string) string {
	return ""
}

func (m empty) Metadata() map[Metadata]string {
	return nil
}

func (m empty) Timestamp() time.Time {
	return time.Time{}
}
