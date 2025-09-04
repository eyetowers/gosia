package sia

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	messageRE = regexp.MustCompile(`^\n([[:xdigit:]]{4})0([[:xdigit:]]{3})"([^"]+)"(\d{4})L\d{1,4}#([[:xdigit:]]{4})\[.*\](?:_\d{2}:\d{2}:\d{2},\d{2}-\d{2}-\d{4})?\r$`)
)

func Parse(msg string) (Message, Identity, uint16, error) {
	matches := messageRE.FindStringSubmatch(msg)
	if len(matches) < 6 {
		return nil, Identity{}, 0, fmt.Errorf("unexpected message format")
	}
	seq, err := parseSequence(matches[4])
	if err != nil {
		return nil, Identity{}, 0, err
	}
	return empty{id: matches[3]}, Identity{AuthCode: matches[5]}, seq, nil
}

func parseSequence(input string) (uint16, error) {
	seq, err := strconv.ParseUint(input, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("malformed sequence number %q: %w", input, err)
	}
	if seq > 9999 {
		return 0, fmt.Errorf("sequence number %d out of range (0-9999): %w", seq, err)
	}
	return uint16(seq), nil
}
