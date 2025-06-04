package sia

import (
	"fmt"
	"time"
)

var codeToSubject = map[string]Subject{
	// AC restoral: AC power has been restored.
	"AR": unspecified,
	// AC trouble: AC power has been failed.
	"AT": unspecified,
	// Burglary alarm: Burglary zone has been violated while armed.
	"BA": zone,
	// Burglary cancel: Alarm has been cancelled.
	"BC": user,
	// Burglary restoral: Alarm/trouble condition eliminated.
	"BR": zone,
}

type field func(m *dcs)

func Zone(id uint16, name string) field {
	return func(m *dcs) {
		m.zone = Identifier{id, name}
	}
}

func Area(id uint16, name string) field {
	return func(m *dcs) {
		m.area = Identifier{id, name}
	}
}

func User(id uint16, name string) field {
	return func(m *dcs) {
		m.user = Identifier{id, name}
	}
}

func Verification(url string) field {
	return func(m *dcs) {
		m.addMetadata(verification, url)
	}
}

func Longitude(lon string) field {
	return func(m *dcs) {
		m.addMetadata(longitude, lon)
	}
}

func Latitude(lat string) field {
	return func(m *dcs) {
		m.addMetadata(latitude, lat)
	}
}

func Altitude(alt string) field {
	return func(m *dcs) {
		m.addMetadata(altitude, alt)
	}
}

func Timestamp(ts time.Time) field {
	return func(m *dcs) {
		m.timestamp = ts
	}
}

func DCS(
	code string,
	fields ...field,
) Message {
	m := dcs{code: code}
	for _, f := range fields {
		f(&m)
	}
	return &m
}

type dcs struct {
	code      string
	zone      Identifier
	area      Identifier
	user      Identifier
	metadata  map[Metadata]string
	timestamp time.Time
}

func (m *dcs) addMetadata(k Metadata, v string) {
	if m.metadata == nil {
		m.metadata = make(map[Metadata]string)
	}
	m.metadata[k] = v
}

func (m dcs) ID() string {
	return "SIA-DCS"
}

func (m dcs) Payload(authCode string) string {
	subject := codeToSubject[m.code]
	result := fmt.Sprintf("#%s|N", authCode)
	result += mayRender("id", m.user, user, subject)
	result += mayRender("ri", m.area, area, subject)
	result += m.code
	switch subject {
	case zone:
		return result + m.zone.Render()
	case area:
		return result + m.area.Render()
	case user:
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

func (m dcs) Metadata() map[Metadata]string {
	return m.metadata
}

func (m dcs) Timestamp() time.Time {
	return m.timestamp
}
