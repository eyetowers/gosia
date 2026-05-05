# gosia
Golang library for alarm system communication using the SIA protocol

Encrypted DC-09 messages are supported with AES-128, AES-192, or AES-256 keys.
Use `Identity.WithEncryptionKeyHex` for the usual hex-encoded receiver keys,
or `Identity.WithEncryptionKey` when you already have raw AES key bytes.
