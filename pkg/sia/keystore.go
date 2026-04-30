package sia

// KeyStore maps account numbers to AES keys for DC-09 encrypted messages.
// Keys must be 16, 24, or 32 bytes (AES-128/192/256).
type KeyStore interface {
	LookupKey(account string) ([]byte, bool)
}

// MapKeyStore is a KeyStore backed by a map.
type MapKeyStore map[string][]byte

func (m MapKeyStore) LookupKey(account string) ([]byte, bool) {
	key, ok := m[account]
	return key, ok
}
