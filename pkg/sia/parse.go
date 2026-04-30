package sia

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// SIA DC-09 framing and grammar errors. Wrap with fmt.Errorf("%w: ...") so
// callers can use errors.Is to assert on the failure mode.
var (
	ErrMalformedFrame  = errors.New("malformed SIA DC-09 frame")
	ErrCRCMismatch     = errors.New("SIA DC-09 CRC mismatch")
	ErrLengthMismatch  = errors.New("SIA DC-09 length mismatch")
	ErrDecryptionFailed = errors.New("SIA DC-09 decryption failed")
)

// ParsedFrame is the structured form of a parsed SIA DC-09 frame.
//
// Per ANSI/SIA DC-09-2013 §5.4.2, the payload grammar is:
//
//	"id"seq[Rrcvr]Lpref[#acct][data][_HH:MM:SS,MM-DD-YYYY]
//
// Receiver and Account are optional on the wire and reported as the empty
// string when absent. Line is required; Sequence is 0-9999.
// Encrypted is set when the frame carried an AES-CBC-encrypted payload
// (DC-09-2013 p.16); the Message ID has the leading '*' stripped.
type ParsedFrame struct {
	Message   Message
	Sequence  uint16
	Receiver  string
	Line      string
	Account   string
	Encrypted bool
}

// Parse validates the framing of a SIA DC-09 message and tokenizes its
// payload into a ParsedFrame. It returns one of ErrMalformedFrame,
// ErrCRCMismatch, or ErrLengthMismatch (wrapped) on failure.
func Parse(msg string) (ParsedFrame, error) {
	payload, err := unframe(msg)
	if err != nil {
		return ParsedFrame{}, err
	}
	return tokenize(payload)
}

// unframe validates the LF<CRC><LLLL><payload>CR envelope per DC-09-2013
// §5.4.1 and verifies the declared length and CRC against the payload.
func unframe(msg string) (string, error) {
	if len(msg) == 0 || msg[0] != '\n' {
		return "", fmt.Errorf("%w: missing leading LF", ErrMalformedFrame)
	}
	if msg[len(msg)-1] != '\r' {
		return "", fmt.Errorf("%w: missing trailing CR", ErrMalformedFrame)
	}
	inner := msg[1 : len(msg)-1]
	if len(inner) < 8 {
		return "", fmt.Errorf("%w: frame too short for CRC and length header", ErrMalformedFrame)
	}
	crc, err := strconv.ParseUint(inner[0:4], 16, 16)
	if err != nil {
		return "", fmt.Errorf("%w: invalid CRC field %q", ErrMalformedFrame, inner[0:4])
	}
	length, err := strconv.ParseUint(inner[4:8], 16, 16)
	if err != nil {
		return "", fmt.Errorf("%w: invalid length field %q", ErrMalformedFrame, inner[4:8])
	}
	payload := inner[8:]
	if len(payload) != int(length) {
		return "", fmt.Errorf("%w: declared %d bytes, got %d", ErrLengthMismatch, length, len(payload))
	}
	actual := checksum([]byte(payload))
	if uint64(actual) != crc {
		return "", fmt.Errorf("%w: declared %04X, computed %04X", ErrCRCMismatch, crc, actual)
	}
	return payload, nil
}

// timestampRE matches the optional trailing "_HH:MM:SS,MM-DD-YYYY" timestamp
// suffix defined in DC-09-2013 §5.4.2.
var timestampRE = regexp.MustCompile(`^\d{2}:\d{2}:\d{2},\d{2}-\d{2}-\d{4}$`)

// headerRE matches the address fields between "id"seq and the first data
// block: optional R<rcvr>, required L<line>, optional #acct.
//
// Line is 1-6 hex digits per DC-09-2013 §5.4.2. Since the only account
// separator defined by DC-09-2013/2021 is '#', a header like "L0A0" is
// interpreted as line "0A0" with no account.
var headerRE = regexp.MustCompile(
	`^(?:R([[:xdigit:]]+))?L([[:xdigit:]]{1,6})(?:#([[:xdigit:]]+))?$`,
)

// tokenize parses an unframed payload using a position cursor for the
// fixed-position prefix ("id" + seq) and trailer (data blocks, timestamp),
// then hands the variable-shape header to headerRE. Splitting it that way
// lets the header regex disambiguate the L/A overlap without backtracking
// across the whole payload.
func tokenize(payload string) (ParsedFrame, error) {
	s := &scanner{src: payload}

	id, err := s.readQuoted()
	if err != nil {
		return ParsedFrame{}, fmt.Errorf("%w: %s", ErrMalformedFrame, err)
	}
	seqStr, err := s.readN(4)
	if err != nil {
		return ParsedFrame{}, fmt.Errorf("%w: reading sequence: %s", ErrMalformedFrame, err)
	}
	seq, err := parseSequence(seqStr)
	if err != nil {
		return ParsedFrame{}, fmt.Errorf("%w: %s", ErrMalformedFrame, err)
	}

	headerEnd := s.pos
	for headerEnd < len(s.src) && s.src[headerEnd] != '[' && s.src[headerEnd] != '_' {
		headerEnd++
	}
	header := s.src[s.pos:headerEnd]
	s.pos = headerEnd

	m := headerRE.FindStringSubmatch(header)
	if m == nil {
		return ParsedFrame{}, fmt.Errorf("%w: malformed address header %q", ErrMalformedFrame, header)
	}
	receiver, line, account := m[1], m[2], m[3]

	if err := scanDataBlocks(s); err != nil {
		return ParsedFrame{}, err
	}

	return ParsedFrame{
		Message:  empty{id: id},
		Sequence: seq,
		Receiver: receiver,
		Line:     line,
		Account:  account,
	}, nil
}

// scanDataBlocks advances s past zero or more "[...]" data blocks and an
// optional "_HH:MM:SS,MM-DD-YYYY" timestamp suffix per DC-09-2013 §5.4.2.
func scanDataBlocks(s *scanner) error {
	for s.peek() == '[' {
		s.advance()
		if !s.skipUntil(']') {
			return fmt.Errorf("%w: unterminated data block", ErrMalformedFrame)
		}
	}
	if s.peek() == '_' {
		s.advance()
		ts := s.rest()
		if !timestampRE.MatchString(ts) {
			return fmt.Errorf("%w: malformed timestamp %q", ErrMalformedFrame, ts)
		}
		s.pos = len(s.src)
	}
	if !s.eof() {
		return fmt.Errorf("%w: trailing data %q", ErrMalformedFrame, s.rest())
	}
	return nil
}

// ParseEncrypted parses a DC-09 frame that may carry an AES-CBC-encrypted
// payload (DC-09-2013 p.16). Encrypted frames have '*' after the opening '"'
// in the ID token (e.g. "*SIA-DCS"); the '*' is stripped from ParsedFrame.Message.ID()
// and ParsedFrame.Encrypted is set to true. Plain frames are handled identically
// to Parse. NAK and DUH are never encrypted and are always parsed as plain.
//
// The account number from the cleartext header is used to look up the AES key
// in keys; ErrDecryptionFailed is returned if no key is found or decryption fails.
func ParseEncrypted(msg string, keys KeyStore) (ParsedFrame, error) {
	payload, err := unframe(msg)
	if err != nil {
		return ParsedFrame{}, err
	}

	// Fast path: no '*' means this is a plain frame.
	if !strings.HasPrefix(payload, "\"*") {
		return tokenize(payload)
	}

	s := &scanner{src: payload}

	id, err := s.readQuoted()
	if err != nil {
		return ParsedFrame{}, fmt.Errorf("%w: %s", ErrMalformedFrame, err)
	}
	if !strings.HasPrefix(id, "*") {
		return tokenize(payload)
	}
	id = id[1:] // strip '*'

	seqStr, err := s.readN(4)
	if err != nil {
		return ParsedFrame{}, fmt.Errorf("%w: reading sequence: %s", ErrMalformedFrame, err)
	}
	seq, err := parseSequence(seqStr)
	if err != nil {
		return ParsedFrame{}, fmt.Errorf("%w: %s", ErrMalformedFrame, err)
	}

	headerEnd := s.pos
	for headerEnd < len(s.src) && s.src[headerEnd] != '[' && s.src[headerEnd] != '_' {
		headerEnd++
	}
	header := s.src[s.pos:headerEnd]
	s.pos = headerEnd

	m := headerRE.FindStringSubmatch(header)
	if m == nil {
		return ParsedFrame{}, fmt.Errorf("%w: malformed address header %q", ErrMalformedFrame, header)
	}
	receiver, line, account := m[1], m[2], m[3]

	if s.peek() != '[' {
		return ParsedFrame{}, fmt.Errorf("%w: expected '[' starting encrypted data block", ErrMalformedFrame)
	}
	s.advance()

	if keys == nil {
		return ParsedFrame{}, fmt.Errorf("%w: no key store configured", ErrDecryptionFailed)
	}
	key, ok := keys.LookupKey(account)
	if !ok {
		return ParsedFrame{}, fmt.Errorf("%w: no key for account %q", ErrDecryptionFailed, account)
	}

	plain, err := decryptPayload(key, s.rest())
	if err != nil {
		return ParsedFrame{}, fmt.Errorf("%w: %s", ErrDecryptionFailed, err)
	}

	// The decrypted content is everything that normally follows '[' in a plain
	// frame: "payload]metadata_timestamp". Prefix it with '[' so scanDataBlocks
	// can process it normally.
	ds := &scanner{src: "[" + string(plain)}
	if err := scanDataBlocks(ds); err != nil {
		return ParsedFrame{}, err
	}

	return ParsedFrame{
		Message:   empty{id: id},
		Sequence:  seq,
		Receiver:  receiver,
		Line:      line,
		Account:   account,
		Encrypted: true,
	}, nil
}

func parseSequence(input string) (uint16, error) {
	seq, err := strconv.ParseUint(input, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("malformed sequence number %q: %w", input, err)
	}
	if seq > 9999 {
		return 0, fmt.Errorf("sequence number %d out of range (0-9999)", seq)
	}
	return uint16(seq), nil
}

type scanner struct {
	src string
	pos int
}

func (s *scanner) eof() bool    { return s.pos >= len(s.src) }
func (s *scanner) advance()     { s.pos++ }
func (s *scanner) rest() string { return s.src[s.pos:] }
func (s *scanner) peek() byte {
	if s.eof() {
		return 0
	}
	return s.src[s.pos]
}

func (s *scanner) readN(n int) (string, error) {
	if s.pos+n > len(s.src) {
		return "", fmt.Errorf("expected %d more bytes, have %d", n, len(s.src)-s.pos)
	}
	out := s.src[s.pos : s.pos+n]
	s.pos += n
	return out, nil
}

func (s *scanner) readQuoted() (string, error) {
	if s.peek() != '"' {
		return "", fmt.Errorf(`expected '"' at offset %d`, s.pos)
	}
	s.advance()
	end := strings.IndexByte(s.rest(), '"')
	if end < 0 {
		return "", fmt.Errorf("unterminated quoted id starting at offset %d", s.pos-1)
	}
	out := s.src[s.pos : s.pos+end]
	if out == "" {
		return "", fmt.Errorf("empty quoted id at offset %d", s.pos-1)
	}
	s.pos += end + 1
	return out, nil
}

func (s *scanner) skipUntil(b byte) bool {
	idx := strings.IndexByte(s.rest(), b)
	if idx < 0 {
		return false
	}
	s.pos += idx + 1
	return true
}
