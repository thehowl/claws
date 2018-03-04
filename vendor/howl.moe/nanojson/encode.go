package nanojson

import (
	"errors"
	"io"
	"unicode/utf8"
)

var errInvalidKind = errors.New("nanojson: invalid Kind of value")

var (
	byteNull  = []byte("null")
	byteTrue  = []byte("true")
	byteFalse = []byte("false")
)

// EncodeJSON encodes v to its JSON representation, and writes the result to w.
func (v *Value) EncodeJSON(w io.Writer) (int, error) {
	vbuf := Pools.EncodeStateBuf.Get()
	e := &encodeState{w: w, buf: vbuf.([]byte)}
	err := e.encodeValue(v)
	if err != nil {
		return 0, err
	}
	err = e.Flush()
	Pools.EncodeStateBuf.Put(vbuf)
	return e.written, err
}

type encodeState struct {
	w       io.Writer
	written int
	pos     byte
	buf     []byte
}

func (e *encodeState) Flush() error {
	if e.pos == 0 {
		return nil
	}
	written, err := e.w.Write(e.buf[:e.pos])
	e.pos = 0
	e.written += written
	return err
}

func (e *encodeState) WriteByte(b byte) error {
	if e.pos == byte(len(e.buf)) {
		err := e.Flush()
		if err != nil {
			return err
		}
	}
	e.buf[e.pos] = b
	e.pos++
	return nil
}

func (e *encodeState) Write2Bytes(b1, b2 byte) error {
	if e.pos >= byte(len(e.buf))-1 {
		err := e.Flush()
		if err != nil {
			return err
		}
	}
	e.buf[e.pos] = b1
	e.buf[e.pos+1] = b2
	e.pos += 2
	return nil
}

// writes to the buffer b, assuming len(b) <= 255
func (e *encodeState) write(b []byte) error {
	if e.pos > byte(len(e.buf)-len(b)) {
		err := e.Flush()
		if err != nil {
			return err
		}
	}
	copy(e.buf[e.pos:e.pos+byte(len(b))], b)
	e.pos += byte(len(b))
	return nil
}

// Write writes b, which can be of arbitrary length.
func (e *encodeState) Write(b []byte) (int, error) {
	if len(b) <= len(e.buf) {
		return len(b), e.write(b)
	}
	err := e.Flush()
	if err != nil {
		return 0, err
	}
	written, err := e.w.Write(b)
	e.written += written
	return written, err
}

func (e *encodeState) encodeValue(v *Value) error {
	switch v.Kind {
	case KindNull:
		return e.write(byteNull)
	case KindTrue:
		return e.write(byteTrue)
	case KindFalse:
		return e.write(byteFalse)

	case KindNumber:
		i, err := e.Write(v.Value)
		e.written += i
		return err

	case KindString:
		return e.encodeString(v.Value)

	case KindArray:
		err := e.WriteByte('[')
		if err != nil {
			return err
		}

		// iterate over children and encode them
		for i := 0; i < len(v.Children); i++ {
			// encode value
			err = e.encodeValue(&v.Children[i])
			if err != nil {
				return err
			}
			// if this is not the last value, write comma
			if i != len(v.Children)-1 {
				err = e.WriteByte(',')
				if err != nil {
					return err
				}
			}
		}

		err = e.WriteByte(']')
		return err

	case KindObject:
		// {
		err := e.WriteByte('{')
		if err != nil {
			return err
		}

		// iterate over children and encode them
		for i := 0; i < len(v.Children); i++ {
			ch := &v.Children[i]
			err = e.encodeString(ch.Key)
			if err != nil {
				return err
			}

			// otherwise, do write comma
			err = e.WriteByte(':')
			if err != nil {
				return err
			}

			// encode value
			err = e.encodeValue(ch)
			if err != nil {
				return err
			}

			// if this is not the last value, write comma
			if i != len(v.Children)-1 {
				err = e.WriteByte(',')
				if err != nil {
					return err
				}
			}
		}

		// }
		err = e.WriteByte('}')
		return err
	}
	return errInvalidKind
}

var (
	slashSlash        = [2]byte{'\\', '\\'}
	slashQuote        = [2]byte{'\\', '"'}
	slashNewline      = [2]byte{'\\', 'n'}
	slashCarriage     = [2]byte{'\\', 'r'}
	slashTabulation   = [2]byte{'\\', 't'}
	slashUnicode      = []byte(`\u00`)
	slashUnicodeError = []byte(`\ufffd`)
	slashU2028        = []byte(`\u2028`)
	slashU2029        = []byte(`\u2029`)
)

var hex = "0123456789abcdef"

// mostly copied from encoding/json/encode.go
func (e *encodeState) encodeString(s []byte) error {
	if len(s) == 0 {
		return e.Write2Bytes('"', '"')
	}
	err := e.WriteByte('"')
	if err != nil {
		return err
	}
	var start int
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if htmlSafeSet[b] {
				i++
				continue
			}
			if start < i {
				_, err = e.Write(s[start:i])
				if err != nil {
					return err
				}
			}
			switch b {
			case '\\', '"':
				err = e.Write2Bytes('\\', b)
			case '\n':
				err = e.Write2Bytes('\\', 'n')
			case '\r':
				err = e.Write2Bytes('\\', 'r')
			case '\t':
				err = e.Write2Bytes('\\', 't')
			default:
				// This encodes bytes < 0x20 except for \t, \n and \r.
				// If escapeHTML is set, it also escapes <, >, and &
				// because they can lead to security holes when
				// user-controlled strings are rendered into JSON
				// and served to some browsers.
				err = e.write(slashUnicode)
				if err != nil {
					return err
				}
				err = e.Write2Bytes(hex[b>>4], hex[b&0xF])
			}
			if err != nil {
				return err
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRune(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				_, err = e.Write(s[start:i])
				if err != nil {
					return err
				}
			}
			err = e.write(slashUnicodeError)
			if err != nil {
				return err
			}
			i++
			start = i
			continue
		}
		// U+2028 is LINE SEPARATOR.
		// U+2029 is PARAGRAPH SEPARATOR.
		// They are both technically valid characters in JSON strings,
		// but don't work in JSONP, which has to be evaluated as JavaScript,
		// and can lead to security holes there. It is valid JSON to
		// escape them, so we do so unconditionally.
		// See http://timelessrepo.com/json-isnt-a-javascript-subset for discussion.
		if c == '\u2028' || c == '\u2029' {
			if start < i {
				err = e.write(s[start:i])
				if err != nil {
					return err
				}
			}
			if c == '\u2028' {
				err = e.write(slashU2028)
			} else {
				err = e.write(slashU2029)
			}
			if err != nil {
				return err
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		_, err = e.Write(s[start:])
		if err != nil {
			return err
		}
	}
	err = e.WriteByte('"')
	return err
}

// htmlSafeSet holds the value true if the ASCII character with the given
// array position can be safely represented inside a JSON string, embedded
// inside of HTML <script> tags, without any additional escaping.
//
// All values are true except for the ASCII control characters (0-31), the
// double quote ("), the backslash character ("\"), HTML opening and closing
// tags ("<" and ">"), and the ampersand ("&").
var htmlSafeSet = [utf8.RuneSelf]bool{
	' ':      true,
	'!':      true,
	'"':      false,
	'#':      true,
	'$':      true,
	'%':      true,
	'&':      false,
	'\'':     true,
	'(':      true,
	')':      true,
	'*':      true,
	'+':      true,
	',':      true,
	'-':      true,
	'.':      true,
	'/':      true,
	'0':      true,
	'1':      true,
	'2':      true,
	'3':      true,
	'4':      true,
	'5':      true,
	'6':      true,
	'7':      true,
	'8':      true,
	'9':      true,
	':':      true,
	';':      true,
	'<':      false,
	'=':      true,
	'>':      false,
	'?':      true,
	'@':      true,
	'A':      true,
	'B':      true,
	'C':      true,
	'D':      true,
	'E':      true,
	'F':      true,
	'G':      true,
	'H':      true,
	'I':      true,
	'J':      true,
	'K':      true,
	'L':      true,
	'M':      true,
	'N':      true,
	'O':      true,
	'P':      true,
	'Q':      true,
	'R':      true,
	'S':      true,
	'T':      true,
	'U':      true,
	'V':      true,
	'W':      true,
	'X':      true,
	'Y':      true,
	'Z':      true,
	'[':      true,
	'\\':     false,
	']':      true,
	'^':      true,
	'_':      true,
	'`':      true,
	'a':      true,
	'b':      true,
	'c':      true,
	'd':      true,
	'e':      true,
	'f':      true,
	'g':      true,
	'h':      true,
	'i':      true,
	'j':      true,
	'k':      true,
	'l':      true,
	'm':      true,
	'n':      true,
	'o':      true,
	'p':      true,
	'q':      true,
	'r':      true,
	's':      true,
	't':      true,
	'u':      true,
	'v':      true,
	'w':      true,
	'x':      true,
	'y':      true,
	'z':      true,
	'{':      true,
	'|':      true,
	'}':      true,
	'~':      true,
	'\u007f': true,
}
