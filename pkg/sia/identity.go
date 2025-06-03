package sia

func AuthCode(code string) Identity {
	return Identity{
		AuthCode: code,
	}
}

type Identity struct {
	AuthCode string
}
