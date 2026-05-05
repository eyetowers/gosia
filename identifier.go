package sia

import (
	"strconv"
)

// Subject identifies the SIA-DCS subject type expected by an event code.
type Subject int

const (
	unspecified Subject = iota
	area        Subject = iota
	zone        Subject = iota
	user        Subject = iota
)

// Identifier is a numeric SIA-DCS subject identifier with an optional display name.
type Identifier struct {
	id   uint16
	name string
}

// Empty reports whether the identifier has no id.
func (i Identifier) Empty() bool {
	return i.id == 0
}

// Render formats the identifier for a SIA-DCS payload.
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
