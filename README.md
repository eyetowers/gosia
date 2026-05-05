# gosia

Go library for alarm system communication using the SIA DC-09 protocol.

`gosia` can encode, parse, send, and acknowledge SIA DC-09 frames. Encrypted
DC-09 messages are supported with AES-128, AES-192, or AES-256 keys.

## Install

```sh
go get github.com/eyetowers/gosia
```

Import the SIA package:

```go
import "github.com/eyetowers/gosia"
```

## Client example

```go
identity := sia.Account("1234")

client, err := sia.Dial(
	"127.0.0.1:5000",
	identity,
	sia.WithKeepalive(30*time.Second),
)
if err != nil {
	return err
}
defer client.Close()

return client.Send(sia.Event(
	"BA",
	sia.Zone(2, "Front Door"),
	sia.Area(1, "Main"),
	sia.Timestamp(time.Now()),
))
```

To use an encrypted receiver key, pass the usual hex-encoded key:

```go
identity, err := sia.Account("1234").WithEncryptionKeyHex("30313233343536373839414243444546")
if err != nil {
	return err
}
```

Use `Identity.WithEncryptionKey` when you already have raw AES key bytes.

The client sends an initial ping when it connects. Periodic keepalive pings are
disabled by default; use `WithKeepalive` to enable them.

## Example programs

The programs in `examples/` are small smoke-test tools, not production
commands.

Run the example receiver:

```sh
go run ./examples/server 127.0.0.1:5000
```

Run it with an encrypted receiver key:

```sh
go run ./examples/server 127.0.0.1:5000 30313233343536373839414243444546
```

Send example events:

```sh
go run ./examples/client 127.0.0.1:5000 1234
```

## Supported features

- DC-09 framing, length checks, and CRC validation.
- Message parsing into account, line, receiver, sequence, and message id.
- SIA-DCS event encoding for common events such as `BA`, `BR`, `CG`, `OA`, and
  `RP`.
- Metadata fields for verification URLs and location values.
- Encrypted DC-09 payloads with AES-128, AES-192, and AES-256 keys.

## Status and limitations

This library focuses on the DC-09 framing and SIA-DCS messages used by this
project. It is not a complete implementation of every SIA event code or every
receiver behavior. Add missing event helpers as needed, and keep protocol
fixtures in tests when changing parser or encoder behavior.

## Development

```sh
go test ./...
go vet ./...
```

## License

MIT License. Copyright (c) 2025 EyeTowers.
