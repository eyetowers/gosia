# Contributing

Thanks for taking the time to improve `gosia`.

## Development

Run the usual Go checks before opening a pull request:

```sh
gofmt -w .
go test ./...
go vet ./...
```

Keep changes focused. Parser and encoder changes should include protocol
fixtures or round-trip tests that show the expected frame behavior.

## Issues and pull requests

When reporting a bug, include the frame, expected behavior, actual behavior,
and whether encryption is involved. Do not include production receiver keys,
account numbers, or customer data.

For pull requests, describe the protocol behavior being changed and note any
compatibility impact for existing callers.
