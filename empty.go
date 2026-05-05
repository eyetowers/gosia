package sia

import "time"

var (
	// Null is the SIA DC-09 heartbeat message.
	Null = empty{id: "NULL"}
	// Ack is the positive receiver acknowledgement message.
	Ack = empty{id: "ACK"}
	// Nak is the negative receiver acknowledgement message.
	Nak = empty{id: "NAK"}
	// Duh is the receiver response for an unsupported or unrecognized message.
	Duh = empty{id: "DUH"}
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
