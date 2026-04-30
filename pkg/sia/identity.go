package sia

func Account(account string) Identity {
	return Identity{
		Account: account,
	}
}

type Identity struct {
	Account string
	Line    string

	encryptionKey []byte
}

func (i Identity) WithEncryptionKey(key []byte) Identity {
	i.encryptionKey = append([]byte(nil), key...)
	return i
}

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
