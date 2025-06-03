package sia

type Message interface {
	Payload(authCode string) string
	ID() string
}
