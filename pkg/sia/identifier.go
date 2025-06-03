package sia

import (
	"strconv"
)

type Subject int

const (
	unspecified Subject = iota
	area        Subject = iota
	zone        Subject = iota
	user        Subject = iota
)

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
