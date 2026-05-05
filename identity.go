package sia

// Account returns an Identity for the given SIA account number.
func Account(account string) Identity {
	return Identity{
		Account: account,
	}
}

// Identity identifies the account and optional line used to send SIA frames.
type Identity struct {
	Account string
	Line    string

	encryptionKey []byte
}

// WithEncryptionKey returns a copy of the identity configured with raw AES key bytes.
func (i Identity) WithEncryptionKey(key []byte) Identity {
	i.encryptionKey = append([]byte(nil), key...)
	return i
}

// WithEncryptionKeyHex returns a copy of the identity configured with a hex-encoded AES key.
func (i Identity) WithEncryptionKeyHex(key string) (Identity, error) {
	parsed, err := ParseEncryptionKey(key)
	if err != nil {
		return i, err
	}
	return i.WithEncryptionKey(parsed), nil
}

func (i Identity) key() []byte {
	return i.encryptionKey
}
