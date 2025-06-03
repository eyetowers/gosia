package sia

import "fmt"

var codeToSubject = map[string]Subject{
	"AR": Unspecified,
	"AT": Unspecified,
	"BA": Zone,
	"BC": User,
}

func DCS(
	code string,
	zone, area, user Identifier,
) Message {
	return &dcs{
		code: code,
		zone: zone,
		area: area,
		user: user,
	}
}

type dcs struct {
	code string
	zone Identifier
	area Identifier
	user Identifier
}

func (m dcs) ID() string {
	return "SIA-DCS"
}

func (m dcs) Payload(authCode string) string {
	subject := codeToSubject[m.code]
	result := fmt.Sprintf("#%s|N", authCode)
	result += mayRender("id", m.user, User, subject)
	result += mayRender("ri", m.area, Area, subject)
	result += m.code
	switch subject {
	case Zone:
		return result + m.zone.Render()
	case Area:
		return result + m.area.Render()
	case User:
		return result + m.user.Render()
	}
	return result
}

func mayRender(tag string, identifier Identifier, expected, actual Subject) string {
	if expected == actual || identifier.Empty() {
		return ""
	}
	return tag + identifier.Render()
}
