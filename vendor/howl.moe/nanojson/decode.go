package nanojson // import "howl.moe/nanojson"

import (
	"encoding"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/valyala/bytebufferpool"
)

// nanojson packs all JSON data types into a Value - to do that, we can
// identify which data type it is by inspecting the Kind field, which will
// have one of the 9 constants below.
const (
	KindInvalid uint8 = iota
	KindString
	KindNumber
	KindObject
	KindArray
	KindTrue
	KindFalse
	KindNull
)

var kindStr = [...]string{
	"value",
	"string",
	"number",
	"object",
	"array",
	"true",
	"false",
	"null",
}

// Value specifies a single value in the JSON data. Value is the raw
// representation of JSON data before it is placed into a Go value. Efficient
// use of a Value should get it and place it from a pool (e.g. Pools.Value), of
// course taking care in resetting it before placing it back.
type Value struct {
	// Kind of the Value. See the constants Kind*.
	Kind uint8

	// Value is filled in the cases of KindNumber and KindString. For
	// KindString, Value is actually the parsed string, with all the slash
	// escaping replaced with their Go representation. Numbers are placed as
	// they are, and parsing of them is left to the user.
	// In encoding, if Kind is KindString, the Value is appropriately escaped.
	// Otherwise, in the case of KindNumber, Value is copied with no operation
	// in-between. (So yes, KindNumber can be used to encode raw JSON data.)
	Value []byte

	// Key is only set if the upper Value is of KindObject - in this case,
	// it will be set to the parsed string of the key.
	Key []byte

	// Children is set in case the Kind is KindObject or KindArray. The children
	// properties are listed in the Children - in the case of KindObject, the
	// children will also have their "Key" field set.
	Children []Value

	// To speed up lookup times, we use a map (of course, placed inside a pool)
	// to quickly see what position the child we're looking for is.
	propertyMap map[string]int

	// Empty interfaces. Used when dealing with pools and to ensure zero-alloc.
	currentValueSlice  interface{}
	virtualPropertyMap interface{}
}

// Reset resets the Value's fields, each to its zero value.
func (v *Value) Reset() {
	v.Kind = 0
	v.Value = nil
	v.Key = nil
	v.Children = nil
	v.propertyMap = nil
	v.currentValueSlice = nil
	v.virtualPropertyMap = nil
}

// Clone creates a copy of a Value that is completely independent of the the
// original. It will still be dependent on the original slice of bytes not being
// changed.
func (v *Value) Clone() *Value {
	x := Pools.Value.Get().(*Value)
	x.Kind = v.Kind
	x.Value = v.Value
	x.Key = v.Key
	// no children - nothing left to do
	if len(x.Children) == 0 {
		return x
	}
	x.Children = make([]Value, len(v.Children))
	for i := 0; i < len(v.Children); i++ {
		x.Children[i] = *(&v.Children[i]).Clone()
	}
	// we let propertyMap be
	return x
}

// Recycle gives all the Children slices back to the pool, recursively, so that
// they can be reused in future parses. Callers must not retain references to
// v.Children or any of its values - if they wish to retain values, they should
// copy them or not call Recycle(). Keeping references to the children's .Value
// or .Key is allowed, as it is a reference to the slice and not actually
// reused.
func (v *Value) Recycle() {
	// Fast path: no children, nothing to do.
	if len(v.Children) == 0 {
		return
	}

	// iterate over children and recycle them recursively if they have children
	// themselves.
	var c *Value
	for i := 0; i < len(v.Children); i++ {
		c = &v.Children[i]
		if c.Children != nil {
			c.Recycle()
		}
		// Don't call recycle - most fields are already handled by Recycle
		// itself.
		c.Kind = 0
		c.Value = nil
		c.Key = nil
	}

	// reset v.Children and give currentValueSlice back to the pool, if any.
	v.Children = nil
	if v.currentValueSlice != nil {
		Pools.ValueSlice.Put(v.currentValueSlice)
		v.currentValueSlice = nil
	}

	// If it's an object, check whether it has a propertyMap - if so, return it
	// to the pool.
	if v.Kind == KindObject && v.propertyMap != nil {
		v.propertyMap = nil
		Pools.PropertyMap.Put(v.virtualPropertyMap)
		v.virtualPropertyMap = nil
	}
}

// Property gets the children element in the object which Key is s, or nil if
// it does not exist.
//
// Property will use an internal property map to find the desired element if
// possible; this is the case of a Value which has been created or modified by
// Parse. If not available, it will iterate through the items. The only issue
// which may arise with this is the case where v.Children has been modified and
// v was created through Parse - in that case, Property may return nil even if
// one of the children does have the desired key. In that case, the user can
// create their own logic for finding the key, which should be pretty trivial.
func (v *Value) Property(s string) *Value {
	if v.Kind != KindObject {
		return nil
	}

	// fast path: we have the property map, so we can look that up directly.
	if v.propertyMap != nil {
		if idx, ok := v.propertyMap[s]; ok && idx < len(v.Children) {
			child := &v.Children[idx]
			if b2s(child.Key) == s {
				return child
			}
		}
		// property map is available but the key was not found - it is not in
		// the object.
		return nil
	}

	// slow path: we don't actually know, so we need to check every value.
	for i := 0; i < len(v.Children); i++ {
		x := &v.Children[i]
		if b2s(x.Key) == s {
			return x
		}
	}
	return nil
}

// decodeState holds the current state being decoded. It mostly wraps around the
// *Value currently being decoded, and holds the byte slice we're reading data
// from as well.
type decodeState struct {
	v *Value

	bs  []byte
	pos int
}

// ParseError is a general error happened during parsing.
type ParseError struct {
	// What were we parsing when the error was found?
	Kind uint8

	// Position in the byte slice passed to Parse, and character that triggered
	// the error.
	Pos  int
	Char byte

	// Proper reason why the error happened.
	Reason string
}

func (d *ParseError) Error() string {
	if d.Kind > KindNull || d.Kind < KindInvalid {
		d.Kind = KindInvalid
	}
	return "nanojson: error while parsing " + kindStr[d.Kind] +
		" at pos " + strconv.Itoa(d.Pos) + " ('" + string(d.Char) +
		"'): " + d.Reason
}

var wsChars = [256]bool{
	' ':  true,
	'\t': true,
	'\r': true,
	'\n': true,
}

// value reads any kind of JSON value from d.bs, starting at pos. Its main job
// is to skip the initial padding (whitespace), determine the type of value,
// and pass it down the correct matching parse* function (or validateNumber in
// the case of numbers), except for null, true and false which it will parse
// itself.
//
// If space is false, the initial check for whitespace characters will be
// skipped.
func (d *decodeState) value(space bool) error {
	b := d.bs[d.pos]

	// If space is true, we iterate over the characters until they are not
	// one of the four allowed whitespace characters.
	if space {
		for wsChars[b] {
			d.pos++
			if d.pos >= len(d.bs) {
				return &ParseError{0, d.pos - 1, b, "no JSON value found"}
			}
			b = d.bs[d.pos]
		}
	}

	// Number: pass it down to validateNumber
	if b >= '0' && b <= '9' || b == '-' {
		return d.validateNumber()
	}

	// Arrays, objects, strings, bools and null all require at least two more
	// bytes.
	if !d.atLeast(2) {
		return &ParseError{0, d.pos, b,
			"not enough bytes to parse another value (need 2)"}
	}

	switch b {
	// Arrays, objects and strings are handled by their respective parse* funcs
	case '[':
		return d.parseArray()
	case '{':
		return d.parseObject()
	case '"':
		return d.parseString()

	// true, null
	case 't', 'n':
		if !d.atLeast(4) {
			return &ParseError{0, d.pos, b,
				"not enough characters for null/true"}
		}
		switch b2s(d.bs[d.pos : d.pos+4]) {
		case "true":
			d.v.Kind = KindTrue
		case "null":
			d.v.Kind = KindNull
		default:
			return &ParseError{0, d.pos, b,
				"expected one of 'true', 'null'"}
		}
		d.pos += 4
		return nil

	// false
	case 'f':
		if !d.atLeast(5) {
			return &ParseError{0, d.pos, b,
				"not enough characters for false"}
		}
		if b2s(d.bs[d.pos:d.pos+5]) != "false" {
			return &ParseError{0, d.pos, b, "expected 'false'"}
		}
		d.v.Kind = KindFalse
		d.pos += 5
		return nil
	}

	return &ParseError{0, d.pos, b, "unhandleable token"}
}

func (d *decodeState) parseString() error {
	d.v.Kind = KindString

	if d.bs[d.pos+1] == '"' {
		// fast path: empty string, we don't need to do any parsing.
		d.pos += 2
		return nil
	}

	// Cursor for the src slice.
	cursor := 0

	// in-place filtering: we overwrite data in the original slice by appending
	// the new bytes.
	src := d.bs[d.pos+1:]
	dst := src[:0]

	var b byte
	for cursor < len(src) {
		b = src[cursor]

		// Quick check on b which the processor will probably easily understand
		// and branch predict. The only special cases we handle are
		// backslashes, quotes and control codes. Control codes in ASCII must
		// have their first 3 bits set to 0, which is why b&0xE0 != 0 works.
		if b != '\\' && b != '"' && b&0xE0 != 0 {
			dst = dst[:len(dst)+1]
			cursor++
			if cursor != len(dst) {
				dst[len(dst)-1] = b
			}
			continue
		}

		switch b {
		// Reject control characters
		default:
			return &ParseError{KindString, d.pos + cursor + 1, b,
				"invalid control character in string"}

		// Quote: end string
		case '"':
			d.pos += cursor + 2
			// Full slice expression allows us to tell GC precisely what parts
			// of the memory we need (thus allowing it to take the others, if
			// needed).
			d.v.Value = dst[0:len(dst):len(dst)]
			return nil

		// backslash: special case
		case '\\':
			// backslash requires at the very least another character
			if len(src)-cursor < 2 {
				return &ParseError{KindString, d.pos + cursor + 1, b,
					"backslash without any other character"}
			}

			cursor++
			b = src[cursor]
			switch b {
			default:
				return &ParseError{KindString, d.pos + cursor + 1, b,
					"invalid backslash escape"}
			case '\\', '/', '"', '\'':
				dst = append(dst, b)
			case 'n':
				dst = append(dst, '\n')
			case 't':
				dst = append(dst, '\t')
			case 'b':
				dst = append(dst, '\b')
			case 'f':
				dst = append(dst, '\f')
			case 'r':
				dst = append(dst, '\r')

			// unicode code point
			case 'u':
				// ensure there are at least four more bytes for the code point.
				if len(src)-cursor < 5 {
					return &ParseError{KindString, d.pos + cursor + 1, b,
						"unicode code point requires 4 hex characters"}
				}

				// skip over 'u'
				cursor++
				// read the 4 hex bytes
				resRaw, err := strconv.ParseUint(b2s(src[cursor:cursor+4]),
					16, 64)
				if err != nil {
					return &ParseError{KindString, d.pos + cursor + 1,
						src[cursor],
						"invalid hex sequence for unicode code point"}
				}
				res := rune(resRaw)
				cursor += 4

				// if the result is actually a surrogate, then we check the
				// \u codepoint after this and make sure they actually make up
				// a code point. otherwise, it's an invalid codepoint, so we use
				// ReplacementChar.
				if utf16.IsSurrogate(res) {
					var ok bool
					if len(src)-cursor >= 6 {
						res, ok = surrogate(res, src[cursor:cursor+6])
					}
					if ok {
						cursor += 6
					} else {
						res = unicode.ReplacementChar
					}
				}

				written := utf8.EncodeRune(dst[len(dst):len(dst)+4], res)
				dst = dst[:len(dst)+written]
				continue
			}
			cursor++
		}
	}

	// no closing "
	return &ParseError{KindString, len(d.bs) - 1, d.bs[len(d.bs)-1],
		"missing closing quote '\"' for string"}
}

// surrogate decodes an unicode code point that is composed in two parts,
// surrogates. Known the first part, r1, surrogates tries to parse the second
// part.
func surrogate(r1 rune, src []byte) (rune, bool) {
	if src[0] != '\\' || src[1] != 'u' {
		return 0, false
	}
	res, err := strconv.ParseUint(b2s(src[2:6]), 16, 64)
	if err != nil {
		return 0, false
	}
	return utf16.DecodeRune(r1, rune(res)), true
}

// atLeast ensures that there are at least n bytes _after_ the current position.
func (d *decodeState) atLeast(n int) bool {
	return len(d.bs)-d.pos >= n
}

func (d *decodeState) validateNumber() error {
	d.v.Kind = KindNumber
	start := d.pos

	// There may be a negative sign at the beginning of the string.
	if d.bs[d.pos] == '-' {
		d.pos++
		if d.pos >= len(d.bs) {
			return &ParseError{KindNumber, d.pos - 1, '-',
				"unterminated number"}
		}
	}
	b := d.bs[d.pos]

	// If it starts with 0, it must have only that. If it starts with any other
	// digit, it may carry on.
	switch {
	case b == '0':
		d.pos++
	case b >= '1' && b <= '9':
		d.pos++
		// skip until we find a non-digit
		for _, b = range d.bs[d.pos:] {
			if b < '0' || b > '9' {
				break
			}
			d.pos++
		}
	default:
		return &ParseError{KindNumber, d.pos, b,
			"invalid character for JSON number"}
	}

	// no more bytes: nothing to do
	if d.pos >= len(d.bs) {
		// Full slice expression allows us to tell GC precisely what parts
		// of the memory we need (thus allowing it to take the others, if
		// needed).
		d.v.Value = d.bs[start:d.pos:d.pos]
		return nil
	}

	b = d.bs[d.pos]

	// full stop: decimal. Must have at least one digit, and then as many as
	// necessary.
	if b == '.' {
		d.pos++
		if d.pos >= len(d.bs) {
			return &ParseError{KindNumber, d.pos - 1, '.',
				"unterminated number"}
		}

		// the first character after a decimal mark must be a digit.
		b = d.bs[d.pos]
		if b < '0' || b > '9' {
			return &ParseError{KindNumber, d.pos, b,
				"invalid character for JSON number"}
		}
		d.pos++

		// iterate over every remaining byte until we find a non-digit
		for _, b = range d.bs[d.pos:] {
			if b < '0' || b > '9' {
				break
			}
			d.pos++
		}

		if d.pos >= len(d.bs) {
			d.v.Value = d.bs[start:d.pos:d.pos]
			return nil
		}
		b = d.bs[d.pos]
	}

	// e/E: scientific notation. May have +/- sign, then at least one digit.
	if b == 'e' || b == 'E' {
		d.pos++
		if d.pos >= len(d.bs) {
			return &ParseError{KindNumber, d.pos - 1, b,
				"unterminated number"}
		}
		b = d.bs[d.pos]
		if b == '+' || b == '-' {
			d.pos++
			if d.pos >= len(d.bs) {
				return &ParseError{KindNumber, d.pos - 1, b,
					"unterminated number"}
			}
			b = d.bs[d.pos]
		}
		if b < '0' || b > '9' {
			return &ParseError{KindNumber, d.pos, b,
				"invalid character for JSON number"}
		}
		d.pos++
		for _, b = range d.bs[d.pos:] {
			if b < '0' || b > '9' {
				break
			}
			d.pos++
		}
	}

	d.v.Value = d.bs[start:d.pos:d.pos]
	return nil
}

func (d *decodeState) parseArray() error {
	d.v.Kind = KindArray

	pos := 1
	bs := d.bs[d.pos:]
	currentValue := d.v

	var b byte

	// iterate over whitespace
	for b = bs[pos]; wsChars[b] && pos < len(bs); b = bs[pos] {
		pos++
	}
	// no element array
	if pos < len(bs) && b == ']' {
		d.pos += pos + 1
		return nil
	}

ArrayLoop:
	for pos < len(bs) {
		b = bs[pos]

		// sync d.pos
		d.pos += pos

		d.v = d.v.newChild()
		if err := d.value(false); err != nil {
			return err
		}
		d.v = currentValue

		// sync pos and bs with the new d.pos and d.bs
		pos = 0
		bs = d.bs[d.pos:]

		if pos >= len(bs) {
			break
		}
		// skip ws
		for b = bs[pos]; wsChars[b]; b = bs[pos] {
			pos++
			if pos >= len(bs) {
				break ArrayLoop
			}
		}

		switch b {
		case ',':
			pos++
			if pos >= len(bs) {
				break ArrayLoop
			}
			for b = bs[pos]; wsChars[b] && pos < len(bs); b = bs[pos] {
				pos++
			}
			continue
		case ']':
			d.pos += pos + 1
			return nil
		default:
			return &ParseError{KindArray, d.pos + pos, b,
				"invalid char after value"}
		}
	}

	return &ParseError{KindArray, d.pos - 1, d.bs[len(d.bs)-1],
		"missing closing bracket ']' for array"}
}

const (
	expectKey = iota
	expectColon
	expectValue
	expectComma
)

var expectStr = [...]string{
	"key",
	"colon",
	"value",
	"comma",
}

func (d *decodeState) parseObject() error {
	d.v.Kind = KindObject

	src := d.bs[d.pos:]
	cursor := 1

	currentValue := d.v

	var status uint8
	var b byte
	for cursor < len(src) {
		b = src[cursor]
		if wsChars[b] {
			cursor++
			continue
		}
		switch b {
		case '"':
			// if we are expecting a key, read as a key. otherwise fallthrough
			// to read as value
			if status == expectKey {
				// increment d.pos and reset cursor (sync with the decodeState)
				d.pos += cursor
				cursor = 0
				// create new child. swap the current value in the decodestate
				// with the child
				ch := d.v.newChild()
				d.v = ch
				// parse key
				if err := d.parseString(); err != nil {
					return err
				}
				// swap value with key.
				ch.Key = ch.Value
				ch.Value = nil
				// we now expect a colon before the value of this property.
				status = expectColon
				// set src to start from the unparsed data in d.bs
				src = d.bs[d.pos:]
				break
			}
			fallthrough
		default:
			// new value
			if status != expectValue {
				return &ParseError{0, d.pos + cursor, b,
					"unexpected value, expecting " + expectStr[status]}
			}
			// increment d.pos and reset cursor (sync with the decodeState)
			d.pos += cursor
			cursor = 0
			// the case '"' has already changed d.v to make it so that it is
			// the last child - so all we need to do on our part is to parse
			// the value for that property in the object.
			if err := d.value(false); err != nil {
				return err
			}
			// hold onto the key for later
			k := d.v.Key
			// change d.v to be the root value back again
			d.v = currentValue
			// first child! We need to create the propertyMap before setting
			// the first key.
			if len(d.v.Children) == 1 {
				d.v.virtualPropertyMap = Pools.PropertyMap.Get()
				d.v.propertyMap = d.v.virtualPropertyMap.(map[string]int)
			}
			// set the key in the propertyMap
			d.v.propertyMap[b2s(k)] = len(d.v.Children) - 1
			// go back to parsing
			status = expectComma
			src = d.bs[d.pos:]

		case ',':
			if status != expectComma {
				return &ParseError{0, d.pos + cursor, b,
					"unexpected comma, expecting " + expectStr[status]}
			}
			status = expectKey
			cursor++
		case ':':
			if status != expectColon {
				return &ParseError{0, d.pos + cursor, b,
					"unexpected colon, expecting " + expectStr[status]}
			}
			status = expectValue
			cursor++

		case '}':
			// close object, only if it's an empty object or if we were waiting
			// for the comma after the last element.
			if (status == expectKey && len(d.v.Children) == 0) ||
				status == expectComma {
				d.pos += cursor + 1
				return nil
			}
			return &ParseError{0, d.pos + cursor, b,
				"can't close object after " + expectStr[status]}
		}
	}

	return &ParseError{0, d.pos - 1, d.bs[len(d.bs)-1],
		"missing closing bracket '}' for object"}
}

func (v *Value) newChild() *Value {
	// slice cap has not been reached yet - grow through reslice.
	if len(v.Children) < cap(v.Children) {
		v.Children = v.Children[:len(v.Children)+1]
		target := &v.Children[len(v.Children)-1]
		return target
	}

	// empty slice - get from pool
	if cap(v.Children) == 0 {
		v.currentValueSlice = Pools.ValueSlice.Get()
		v.Children = v.currentValueSlice.([]Value)[:1]
		target := &v.Children[0]
		return target
	}

	// v.Children is too big to fit in an array from the pool - we leave it to
	// append to manage.
	// first, we create a copy of v.Children. This will be used to reset all
	// of the children while leaving intact the others.
	prev := v.Children
	v.Children = append(v.Children, Value{})
	if v.currentValueSlice != nil {
		// make sure that currentValueSlice will not hold any reference to any
		// "dead" object (only referenced in the pool but which for that reason
		// can't be picked up by GC)
		for i := 0; i < len(prev); i++ {
			(&prev[i]).Reset()
		}
		Pools.ValueSlice.Put(v.currentValueSlice)
		v.currentValueSlice = nil
	}
	target := &v.Children[len(v.Children)-1]
	return target
}

// LeftoverError is returned by Parse when the given data is more than just the
// expected JSON value to be parsed, and any additional trailing whitespace
// in the set of ' ', '\t', '\n', '\r'. It will contain the additional data,
// and the error itself will tell the amount of bytes left over.
type LeftoverError []byte

func (e *LeftoverError) Error() string {
	return "nanojson: " + strconv.Itoa(len(*e)) + " leftover bytes after value"
}

var errBIsEmpty = errors.New("nanojson: data or b is empty")

// Parse parses a single value from the Reader. To ensure zero allocation, Parse
// reserves the right to modify b's content (e.g. for parsing strings); for this
// reason, you must create a copy to pass to b in case you wish to retain the
// original.
//
// It is also important to note that v will hold references to parts of b,
// therefore v's content is only valid as long as b is not modified.
//
// Parse will read only the first value inside of b, and expects the rest to be
// exclusively whitespace. If anything else is found, then an error of type
// LeftoverError is returned.
func (v *Value) Parse(b []byte) error {
	if len(b) == 0 {
		return errBIsEmpty
	}

	// initialise decode state
	s := &decodeState{
		v:  v,
		bs: b,
	}

	err := s.value(true)
	if err != nil {
		return err
	}
	// Ensure that everything after this is just whitespace.
	for _, b := range s.bs[s.pos:] {
		if !wsChars[b] {
			err := LeftoverError(s.bs[s.pos:])
			return &err
		}
	}
	return nil
}

/*******************************************************************************
High-level unmarshaling functions
*******************************************************************************/

// UnmarshalOptions specifies the options for parsing JSON data in nanojson.
type UnmarshalOptions struct {
	// By default (false), Unmarshal will copy its data parameter to a new array
	// - that is because the caller might want to retain the original data,
	// whereas Value.Parse in nanojson actually rewrites the original byte
	// slice. Set to true if you don't care if we touch your data parameter.
	DisableDataCopy bool

	// When assigning a Value to a []byte, normally it is enough to do a simple
	// assignment which points at the reference in the original data array.
	// If CopyData is true, however, the value is copied over.
	// It only makes sense to set this to true if DisableDataCopy is also true.
	// (An example of appropriate use would be if you use a []byte from a pool
	// to pass to Unmarshal and you want to retain the struct in which you
	// Unmarshal for long after you give back the []byte to the pool. This would
	// ensure that the data in the struct doesn't become invalid.)
	// An important note: when the destination is a string the value is always
	// copied over regardless.
	CopyData bool
}

var defaultOptions = &UnmarshalOptions{}

// Unmarshal is a shorthand functions to call UnmarshalOptions.Unmarshal with
// the options being their zero value. For more information about the
// unmarshalling process, refer to the documentation of
// UnmarshalOptions.Unmarshal.
func Unmarshal(data []byte, v interface{}) error {
	return defaultOptions.Unmarshal(data, v)
}

var errNotAPointer = errors.New("nanojson: v is not a pointer to a value")
var errNotAddressable = errors.New("nanojson: v is not addressable")
var errVIsNil = errors.New("nanojson: v is nil")

// Unmarshal will parse the first valid JSON value inside of data, and attempt
// the best it can to unmarshal it into v. If v is not a pointer or if v is nil,
// then Unmarshal will return an error. Unmarshal is not exactly
// backwards-compatible with the encoding/json equivalent, so some changes may
// be necessary, but the process should be rather painless if not using
// interfaces.
//
// If after the first value in data, there are more non-whitespace bytes, then
// an error of type LeftoverError is returned, and unmarshaling is interrupted.
//
// In unmarshaling itself, this leads to some kind of loose typing, and some
// cases will be automatically converted. Specifically:
//
//   bool: true if Kind == KindTrue, a string containing only "true", or a non-0
//         number. Rejected if it's an array, object or null.
//   numbers: strconv.ParseInt/Uint/Float, even if the JSON is a string. Will
//            return an error if the strconv functions return one, or if it
//            overflows the Go value.
//   string, []byte, [X]byte: the JSON string, or the raw unparsed number. Empty
//                            otherwise.
//   slices, arrays: will convert each child into its Go representation, except
//                   if the element is uint8/byte. (see above)
//   maps: rejected if key is not a string or JSON is not an object. Will set
//         each key to match the converted JSON value.
//   interface: if *Value implements the interface, then a clone of the original
//              *Value will be assigned to it. Otherwise, it is rejected.
//   struct: matching like encoding/json, except it's case sensitive.
//
// The biggest difference to note here is that the unmarshaling process will not
// automatically create a Go value for you when you specify interface{}. On the
// contrary - it will simply set it to a *nanojson.Value, and you will have to
// take care of handling the value dynamically. Note that in this case, as well
// as in the case of having a field in your struct which is a Value or *Value,
// to ensure data integrity the Value must be cloned first, which incurs in a
// costly alloc+copy (especially if the element has children!).
//
// So what should you do when you need to handle a JSON value dynamically?
// Implement the Unmarshaler interface in a type you define. This way, the
// *Value will not be cloned, instead it will be passed by reference - and it
// will be your burden to ensure data integrity.
//
// Unmarshal is also backwards-compatible with json.Unmarshaler - it is
// important to note, though, that since the parsing process of nanojson
// involves even rewriting the original byte slice to decode strings, that this
// will incur in an EncodeJSON to a temporary buffer before calling
// UnmarshalJSON.
func (u *UnmarshalOptions) Unmarshal(data []byte, v interface{}) error {
	// Options want us to create a copy first
	if !u.DisableDataCopy {
		cp := make([]byte, len(data))
		copy(cp, data)
		data = cp
	}

	// Obtain the value from the Pool.
	vval := Pools.Value.Get()
	val := vval.(*Value)

	// Parse data.
	err := val.Parse(data)
	if err != nil {
		return err
	}

	// Get the value of the interface. If it's not a pointer, error.
	// If it's otherwise not addressable, error.
	x := reflect.ValueOf(v)
	if x.Kind() != reflect.Ptr {
		return errNotAPointer
	}
	if x.IsNil() {
		return errVIsNil
	}
	x = x.Elem()
	if !x.CanSet() {
		return errNotAddressable
	}

	// pass x to toGoValue to do the hard work.
	err = val.toGoValue(x, u, false)
	if err != nil {
		return err
	}

	// dispose of the Value
	val.Recycle()
	val.Reset()
	Pools.Value.Put(vval)
	return nil
}

type stdlibUnmarshaler interface {
	UnmarshalJSON([]byte) error
}

// Unmarshaler is the interface of types capable of unmarhshaling a description
// of themselves as nanojson.Values. UnmarshalValue, if retaining the Value,
// should always clone it and never hold the reference beyond its lifespan.
type Unmarshaler interface {
	UnmarshalValue(v *Value) error
}

type invalidMapping struct {
	from uint8
	to   reflect.Kind
}

func (m *invalidMapping) Error() string {
	if m.from > KindNull || m.from < KindInvalid {
		m.from = KindInvalid
	}
	return "nanojson: invalid mapping of JSON " + kindStr[m.from] +
		" to Go " + m.to.String()
}

var errOverflows = errors.New("nanojson: JSON value overflows type of matching value")
var valueType = reflect.TypeOf(&Value{})
var textUnmarshaler = reflect.ValueOf(struct{ A encoding.TextUnmarshaler }{}).
	Field(0).Type()
var stdlibUnmarshalerType = reflect.ValueOf(struct{ A stdlibUnmarshaler }{}).
	Field(0).Type()
var unmarshalerType = reflect.ValueOf(struct{ A Unmarshaler }{}).Field(0).Type()

func (v *Value) toGoValue(
	x reflect.Value,
	opts *UnmarshalOptions,
	str bool,
) error {
	var t, ptrToT reflect.Type

Begin:
	t = x.Type()
	ptrToT = reflect.PtrTo(t)

	switch {
	// break the switch. Has no methods - cannot implement anything.
	case ptrToT.NumMethod() == 0:
		break

	// t is *Value, we can set it by simply cloning
	case t == valueType:
		x.Set(reflect.ValueOf(v.Clone()))
		return nil
	// t is Value, we need to dereference the value obtained from clone.
	case ptrToT == valueType:
		// We take the addr of x, so we can use it properly
		x.Set(reflect.ValueOf(*v.Clone()))
		return nil

	case ptrToT.Implements(unmarshalerType):
		return x.Addr().Interface().(Unmarshaler).UnmarshalValue(v)
	case v.Kind == KindString && ptrToT.Implements(textUnmarshaler):
		return x.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(v.Value)
	case ptrToT.Implements(stdlibUnmarshalerType):
		// encode value to temporary buffer and pass it on, return error
		bb := bytebufferpool.Get()
		// EncodeJSON returns an error only if the writer returns an error,
		// which it never does, so no need to check.
		v.EncodeJSON(bb)
		var err error
		// If x is nil, we can expect UnmarshalJSON to often panic, so we create
		// it first.
		err = x.Addr().Interface().(stdlibUnmarshaler).UnmarshalJSON(bb.B)
		bytebufferpool.Put(bb)
		return err
	}

	switch t.Kind() {
	case reflect.Ptr:
		if isNull(v, str) {
			x.Set(reflect.Zero(t))
			return nil
		}
		if x.IsNil() {
			x.Set(reflect.New(t.Elem()))
		}
		x = x.Elem()
		// I know what you are thinking. But wrapping the switch in a for loop
		// where we use continue here and place return at the end of the loop
		// for all other branches? No, thanks.
		// We could also use this as a recursive function, but a goto is much
		// simpler, faster and doesn't increase the stack.
		goto Begin

	case reflect.Bool:
		switch {
		case v.Kind == KindTrue, v.Kind == KindFalse:
			x.SetBool(v.Kind == KindTrue)
		case isNull(v, str): // no-op
		default:
			return &invalidMapping{v.Kind, reflect.Bool}
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch {
		case v.Kind == KindNumber, str && v.Kind == KindString: // carry on
		case isNull(v, str): // no-op
			return nil
		default:
			return &invalidMapping{v.Kind, t.Kind()}
		}
		i, err := strconv.ParseInt(b2s(v.Value), 10, 64)
		if err != nil {
			return err
		}
		if !x.OverflowInt(i) {
			x.SetInt(i)
		} else {
			return errOverflows
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64:
		switch {
		case v.Kind == KindNumber, str && v.Kind == KindString: // carry on
		case isNull(v, str): // no-op
			return nil
		default:
			return &invalidMapping{v.Kind, t.Kind()}
		}
		i, err := strconv.ParseUint(b2s(v.Value), 10, 64)
		if err != nil {
			return err
		}
		if !x.OverflowUint(i) {
			x.SetUint(i)
		} else {
			return errOverflows
		}
	case reflect.Float32, reflect.Float64:
		switch {
		case v.Kind == KindNumber, str && v.Kind == KindString: // carry on
		case isNull(v, str): // no-op
			return nil
		default:
			return &invalidMapping{v.Kind, t.Kind()}
		}
		i, err := strconv.ParseFloat(b2s(v.Value), 64)
		if err != nil {
			return err
		}
		if !x.OverflowFloat(i) {
			x.SetFloat(i)
		} else {
			return errOverflows
		}

	case reflect.Array:
		if isNull(v, str) {
			return nil
		}
		// string/numbers and it's a [x]byte: copy v.Value
		if (v.Kind == KindString || v.Kind == KindNumber) &&
			t.Elem().Kind() == reflect.Uint8 {
			reflect.Copy(x, reflect.ValueOf(v.Value))
			return nil
		}
		if v.Kind != KindArray {
			return &invalidMapping{v.Kind, reflect.Array}
		}
		// iterate over each children and copy as value.
		children := v.Children
		if t.Len() < len(children) {
			children = children[:t.Len()]
		}
		for i := 0; i < len(children); i++ {
			err := (&children[i]).toGoValue(x.Index(i), opts, false)
			if err != nil {
				return err
			}
		}
	case reflect.Slice:
		if isNull(v, str) {
			x.Set(reflect.Zero(t))
			return nil
		}
		if (v.Kind == KindString || v.Kind == KindNumber) &&
			t.Elem().Kind() == reflect.Uint8 {
			if opts.CopyData {
				dst := make([]byte, len(v.Value))
				copy(dst, v.Value)
				x.SetBytes(dst)
			} else {
				x.SetBytes(v.Value)
			}
			return nil
		}

		if v.Kind != KindArray {
			return &invalidMapping{v.Kind, reflect.Slice}
		}

		// Check that x has enough elements for the number of Children. If not,
		// grow it.
		if x.Cap() < len(v.Children) {
			dst := reflect.MakeSlice(t, len(v.Children), len(v.Children))
			reflect.Copy(dst, x)
			x.Set(dst)
		} else {
			x.SetLen(len(v.Children))
		}

		for i := 0; i < len(v.Children); i++ {
			err := (&v.Children[i]).toGoValue(x.Index(i), opts, false)
			if err != nil {
				return err
			}
		}

	case reflect.String:
		// encoding.json here, if str is true, would decode the string INSIDE
		// the string, however that's nonsensical for us, if you need that then
		// my boy you have a serious technical debt, but you should instead use
		// a custom unmarshaler instead of hoping we handle your use case.
		switch {
		case isNull(v, str):
			return nil
		case v.Kind != KindString:
			return &invalidMapping{v.Kind, reflect.String}
		}
		// We set it using string conv without b2s, even if the caller asks us
		// kindly. b2s is safe only as long as a reference to the original
		// []byte is kept - thus using it in this case is inadequate.
		x.SetString(string(v.Value))

	case reflect.Map:
		if isNull(v, str) {
			x.Set(reflect.Zero(t))
		}
		// Parse only if it's an object and we have map[string]T.
		if v.Kind != KindObject || t.Key().Kind() != reflect.String {
			return &invalidMapping{v.Kind, reflect.Map}
		}
		if x.IsNil() {
			x.Set(reflect.MakeMapWithSize(t, len(v.Children)))
		}
		elemType := t.Elem()
		for i := 0; i < len(v.Children); i++ {
			dst := reflect.New(elemType).Elem()
			err := (&v.Children[i]).toGoValue(dst, opts, false)
			if err != nil {
				return err
			}
			// It's safe to use b2s here because the original string is not
			// referenced - in fact, maps with strings as the key will hash the
			// string.
			x.SetMapIndex(reflect.ValueOf(b2s(v.Children[i].Key)), dst)
		}

	case reflect.Interface:
		if isNull(v, str) {
			x.Set(reflect.Zero(t))
		}
		if valueType.Implements(t) {
			// We simply set a nanojson.Value. We don't do any
			// map[string]interface{} trickery.
			x.Set(reflect.ValueOf(v.Clone()))
		} else {
			return &invalidMapping{v.Kind, reflect.Interface}
		}
	case reflect.Struct:
		if isNull(v, str) {
			x.Set(reflect.Zero(t))
		}
		// Make sure it's an object.
		if v.Kind != KindObject {
			return &invalidMapping{v.Kind, reflect.Struct}
		}

		// Get the mappings for this struct's fields - generate them if we
		// don't have them.
		typePtr := typeDefPtr(t)
		structMappingsMtx.RLock()
		mappings, ok := structMappings[typePtr]
		structMappingsMtx.RUnlock()
		if !ok {
			mappings = generateMappings(typePtr, t)
		}

		return v.toStruct(x, mappings, opts)
	}
	// In case of unhandleable types (funcs, chans...) we silently ignore them.
	return nil
}

func isNull(v *Value, str bool) bool {
	return v.Kind == KindNull || (str && v.Kind == KindString && b2s(v.Value) == "null")
}

func (v *Value) toStruct(
	x reflect.Value,
	mappings []mapping,
	opts *UnmarshalOptions,
) error {
	nfields := x.NumField()
	var f reflect.Value
	for fIdx := 0; fIdx < nfields; fIdx++ {
		// get the current field - ensure we can modify it
		f = x.Field(fIdx)
		if !f.CanSet() {
			// Can't do anything - skip over.
			continue
		}

		// get its mapping. if it's an embedded field, recursively call
		// toStruct.
		m := mappings[fIdx]
		if m.flags&flagEmbedded != 0 {
			err := v.toStruct(f, m.children, opts)
			if err != nil {
				return err
			}
			continue
		}

		prop := v.Property(m.toProperty)
		err := prop.toGoValue(f, opts, m.flags&flagString != 0)
		if err != nil {
			return err
		}
	}
	return nil
}

const (
	flagEmbedded = 1 << iota
	flagString
	flagOmitempty
)

type mapping struct {
	toProperty string
	children   []mapping
	flags      uint8
}

// structMappings holds the mappings of fields to the property in the expected
// JSON object. Generating a mapping in advance and caching it is insanely
// faster than generating a mapping every time. Type.Field(int) in reflect
// requires an allocation - in fact, it is the only place in Type where an
// allocation is required. Furthermore, parsing struct tags happens to be a very
// expensive operation - thus caching really is the best way to go.
var structMappings = make(map[uintptr][]mapping)
var structMappingsMtx = new(sync.RWMutex)

// typeDefPtr creates an unique identifier to a certain type, having its
// reflect.Type. reflect.Type is the publicly exported version of
// *reflect.rtype, so typeDefPtr gets the pointer to that *reflect.rtype.
//
// This is probably a black box for you and may seem like black magic, however
// there is a rationale for this.
//
//  https://research.swtch.com/interfaces
//  https://blog.golang.org/laws-of-reflection
//  https://github.com/golang/go/blob/master/src/reflect/type.go#L296-L308
//  https://github.com/golang/go/blob/master/src/reflect/type.go#L1410-L1413
func typeDefPtr(t reflect.Type) uintptr {
	return (*[2]uintptr)(unsafe.Pointer(&t))[1]
}

// generateMappings creates mappings for a struct t. It will be placed inside
// of structMappings with the key u.
func generateMappings(u uintptr, t reflect.Type) []mapping {
	// get number of fields and generate destination slice.
	nfields := t.NumField()
	dst := make([]mapping, nfields)

	var f reflect.StructField
	for fIdx := 0; fIdx < nfields; fIdx++ {
		// get the current field.
		f = t.Field(fIdx)
		// ignore unexported fields
		if f.Name[0] < 'A' || f.Name[0] > 'Z' {
			continue
		}

		// if it's embedded, make sure it's a struct and get the mappings
		// for it or generate them if they do not exist.
		if f.Anonymous {
			if f.Type.Kind() != reflect.Struct {
				continue
			}
			tdptr := typeDefPtr(f.Type)
			structMappingsMtx.RLock()
			children, ok := structMappings[tdptr]
			structMappingsMtx.RUnlock()
			if !ok {
				children = generateMappings(tdptr, f.Type)
			}
			dst[fIdx] = mapping{
				flags:    flagEmbedded,
				children: children,
			}
			continue
		}

		// Create new mapping. Use the tag if we have it, and set the right
		// flags. If no name is available, then we use the field name.
		var m mapping
		if tag := f.Tag.Get("json"); tag != "" {
			pos := lastIndexByte(tag, ',')
			if pos < 0 {
				pos = len(tag)
			} else {
				switch tag[pos+1:] {
				case "string":
					m.flags |= flagString
				case "omitempty":
					m.flags |= flagOmitempty
				}
			}
			m.toProperty = tag[:pos]
		}
		if m.toProperty == "" {
			m.toProperty = f.Name
		}
		dst[fIdx] = m
	}
	structMappingsMtx.Lock()
	structMappings[u] = dst
	structMappingsMtx.Unlock()
	return dst
}

// lastIndexByte finds the last position of a given byte in the string s.
// It is copied from strings.LastIndexByte, in order to avoid the unnecessary
// import. You know, "a little copying is better than a little dependency."
func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// b2s converts byte slice to a string without memory allocation.
// See this:
// https://groups.google.com/forum/#!msg/Golang-Nuts/ENgbUzYvCuU/90yGx7GUAgAJ
//
// Note it may break if string and/or slice header will change
// in the future go versions.
func b2s(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}
