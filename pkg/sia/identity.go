package sia

func Account(account string) Identity {
	return Identity{
		Account: account,
	}
}

type Identity struct {
	Account string
	Line    string
}
