package sia

import (
	"strconv"
)

type Subject int

const (
	Unspecified Subject = iota
	Area        Subject = iota
	Zone        Subject = iota
	User        Subject = iota
)

func Empty() Identifier {
	return Identifier{}
}

func Numbered(id uint16) Identifier {
	return Identifier{id: id}
}

func Named(id uint16, name string) Identifier {
	return Identifier{id: id, name: name}
}

type Identifier struct {
	id   uint16
	name string
}

func (i Identifier) Empty() bool {
	return i.id == 0
}

func (i Identifier) Render() string {
	if i.Empty() {
		return ""
	}
	result := strconv.FormatUint(uint64(i.id), 10)
	if i.name == "" {
		return result
	}
	return result + "^" + i.name + "^"
}
