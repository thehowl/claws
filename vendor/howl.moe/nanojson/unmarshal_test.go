package nanojson

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/buger/jsonparser"
)

func TestUnmarshal(t *testing.T) {
	t.Run("Bool", func(t *testing.T) {
		var i bool
		err := Unmarshal([]byte("true"), &i)
		equal(t, err, nil)
		equal(t, i, true)
	})

	t.Run("Int", func(t *testing.T) {
		var i int64
		err := Unmarshal([]byte("13377331"), &i)
		equal(t, err, nil)
		equal(t, i, int64(13377331))
	})

	t.Run("Uint", func(t *testing.T) {
		var i uint64
		err := Unmarshal([]byte("13377331"), &i)
		equal(t, err, nil)
		equal(t, i, uint64(13377331))
	})

	t.Run("IntPtr", func(t *testing.T) {
		var i *int64
		err := Unmarshal([]byte("13377331"), &i)
		equal(t, err, nil)
		equal(t, *i, int64(13377331))
	})
	t.Run("IntPtrNull", func(t *testing.T) {
		var i *int64
		err := Unmarshal([]byte("null"), &i)
		equal(t, err, nil)
		equal(t, i, (*int64)(nil))
	})
	t.Run("Float64", func(t *testing.T) {
		var i float64
		err := Unmarshal([]byte("3.14159e+3"), &i)
		equal(t, err, nil)
		equal(t, i, 3.14159e+3)
	})

	t.Run("Interface", func(t *testing.T) {
		var i interface{}
		err := Unmarshal([]byte("13377331"), &i)
		equal(t, err, nil)
		equal(t, i, &Value{Kind: KindNumber, Value: []byte("13377331")})
	})

	t.Run("[20]byte", func(t *testing.T) {
		var i [20]byte
		// I might have recently seen Singin' in the Rain
		err := Unmarshal([]byte(`"Good mornin' to you"`), &i)
		equal(t, err, nil)
		equal(t, i, [20]byte{'G', 'o', 'o', 'd', ' ', 'm', 'o', 'r', 'n',
			'i', 'n', '\'', ' ', 't', 'o', ' ', 'y', 'o', 'u', 0})
	})
	t.Run("[]byte", func(t *testing.T) {
		var i []byte
		err := Unmarshal([]byte(`"Good mornin' to you"`), &i)
		equal(t, err, nil)
		equal(t, i, []byte("Good mornin' to you"))
	})
	t.Run("string", func(t *testing.T) {
		var i string
		err := Unmarshal([]byte(`"Good mornin' to you"`), &i)
		equal(t, err, nil)
		equal(t, i, "Good mornin' to you")
	})

	t.Run("[]int", func(t *testing.T) {
		dst := make([]int, 5, 5)
		dst[0] = 5511
		err := Unmarshal([]byte(`[1, 2, 3, 4]`), &dst)
		equal(t, err, nil)
		equal(t, cap(dst), 5)
		equal(t, dst, []int{1, 2, 3, 4})
	})
	t.Run("[4]int", func(t *testing.T) {
		var dst [4]int
		err := Unmarshal([]byte(`[1, 2, 3, 4, 5]`), &dst)
		equal(t, err, nil)
		equal(t, dst, [4]int{1, 2, 3, 4})
	})
	t.Run("[]intNil", func(t *testing.T) {
		var dst []int
		err := Unmarshal([]byte(`[1, 2, 3, 4]`), &dst)
		equal(t, err, nil)
		equal(t, cap(dst), 4)
		equal(t, dst, []int{1, 2, 3, 4})
	})

	t.Run("map[string]int", func(t *testing.T) {
		dst := make(map[string]int, 3)
		dst["k3"] = 1337
		err := Unmarshal([]byte(`{"k1": 32, "k2": 33}`), &dst)
		equal(t, err, nil)
		equal(t, len(dst), 3)
		equal(t, dst["k1"], 32)
		equal(t, dst["k2"], 33)
		equal(t, dst["k3"], 1337)
	})
	t.Run("map[string]intNil", func(t *testing.T) {
		var dst map[string]int
		err := Unmarshal([]byte(`{"k1": 32}`), &dst)
		equal(t, err, nil)
		equal(t, len(dst), 1)
		equal(t, dst["k1"], 32)
	})
	t.Run("map[struct{a string}]int", func(t *testing.T) {
		var dst map[struct{ a string }]int
		err := Unmarshal([]byte(`{"k1": 32}`), &dst)
		_ = err.(*invalidMapping)
	})
	t.Run("struct{A string}", func(t *testing.T) {
		var dst struct{ A string }
		err := Unmarshal([]byte(`{"A": "test"}`), &dst)
		equal(t, err, nil)
		equal(t, dst, struct{ A string }{"test"})
	})
	t.Run("struct{B string `json:a`}", func(t *testing.T) {
		type T struct {
			B string `json:"a,string"`
		}
		var dst T
		err := Unmarshal([]byte(`{"a": "uwu"}`), &dst)
		equal(t, err, nil)
		equal(t, dst, T{"uwu"})
	})
	t.Run("structUnexported", func(t *testing.T) {
		type T struct {
			b string
		}
		var dst T
		err := Unmarshal([]byte(`{"b": "uwu"}`), &dst)
		equal(t, err, nil)
		equal(t, dst, T{""})
	})
	t.Run("structWithInt", func(t *testing.T) {
		var dst struct{ B int }
		err := Unmarshal([]byte(`1337`), &dst)
		_ = err.(*invalidMapping)
	})
	t.Run("EmbeddedStruct", func(t *testing.T) {
		type c struct{ C string }
		type BStr struct{ B string }
		type A struct {
			A int
			BStr
			c
		}
		var a A
		err := Unmarshal([]byte(`{"B": "test", "A": 1337, "C": "haha"}`), &a)
		equal(t, err, nil)
		equal(t, a.A, 1337)
		equal(t, a.B, "test")
		equal(t, a.C, "")
	})
	t.Run("Time", func(t *testing.T) {
		orig := time.Date(2007, 11, 10, 23, 11, 11, 0, time.UTC)
		var a struct {
			A time.Time
		}
		err := Unmarshal([]byte(`{"A": "2007-11-10T23:11:11Z"}`), &a)
		equal(t, err, nil)
		equal(t, a.A.Equal(orig), true)
	})
	t.Run("JSONUnmarshaler", func(t *testing.T) {
		var a struct {
			A json.RawMessage
		}
		err := Unmarshal([]byte(`{"A": ["Ha", "Ha"]}`), &a)
		equal(t, err, nil)
		equal(t, string(a.A), `["Ha","Ha"]`)
	})
	t.Run("JSONUnmarshalerWithNull", func(t *testing.T) {
		var a struct {
			A json.RawMessage
		}
		err := Unmarshal([]byte(`{"A": null}`), &a)
		equal(t, err, nil)
		equal(t, string(a.A), `null`)
	})
	t.Run("*JSONUnmarshaler", func(t *testing.T) {
		var a struct {
			A *json.RawMessage
		}
		err := Unmarshal([]byte(`{"A": ["Ha", "Ha"]}`), &a)
		equal(t, err, nil)
		equal(t, string(*a.A), `["Ha","Ha"]`)
	})
	t.Run("*JSONUnmarshalerWithNull", func(t *testing.T) {
		var a struct {
			A *json.RawMessage
		}
		err := Unmarshal([]byte(`{"A": null}`), &a)
		equal(t, err, nil)
		equal(t, a.A, (*json.RawMessage)(nil))
	})
	t.Run("JSONUnmarshalerWithValueReceiver", func(t *testing.T) {
		b := make(byteUnmarshaler, 5)
		err := Unmarshal([]byte(`"Hello"`), &b)
		equal(t, err, nil)
		equal(t, string(b), "Hello")
	})
	t.Run("Unmarshaler", func(t *testing.T) {
		var x testUnmarshaler
		err := Unmarshal([]byte(`"Hello World"`), &x)
		equal(t, err, nil)
		equal(t, string(x), "Hello World")
	})
	t.Run("Value", func(t *testing.T) {
		var r struct{ V Value }
		err := Unmarshal([]byte(`{"V": true}`), &r)
		equal(t, err, nil)
		equal(t, r.V.Kind, KindTrue)
		equal(t, r.V.Key, []byte{'V'})
	})
}

type byteUnmarshaler []byte

func (b byteUnmarshaler) UnmarshalJSON(data []byte) error {
	copy(b, data[1:len(data)-1])
	return nil
}

type testUnmarshaler string

func (t *testUnmarshaler) UnmarshalValue(v *Value) {
	*t = testUnmarshaler(v.Value)
}

func BenchmarkUnmarshalIntoStruct(b *testing.B) {
	data := []byte(`{
    "st": 1,
    "sid": 486,
    "tt": "active",
    "gr": 0,
    "uuid": "de305d54-75b4-431b-adb2-eb6b9e546014",
    "ip": "127.0.0.1",
    "ua": "user_agent",
    "tz": -6,
    "v": 1
}`)
	b.SetBytes(int64(len(data)))
	var dst struct {
		St   int    `json:"st"`
		SID  int    `json:"sid"`
		TT   []byte `json:"tt"`
		Gr   int    `json:"gr"`
		UUID []byte `json:"uuid"`
	}
	opts := &UnmarshalOptions{
		DisableDataCopy: true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opts.Unmarshal(data, &dst)
	}
}

func BenchmarkManualUnmarshalIntoStruct(b *testing.B) {
	data := []byte(`{
    "st": 1,
    "sid": 486,
    "tt": "active",
    "gr": 0,
    "uuid": "de305d54-75b4-431b-adb2-eb6b9e546014",
    "ip": "127.0.0.1",
    "ua": "user_agent",
    "tz": -6,
    "v": 1
}`)
	b.SetBytes(int64(len(data)))
	var dst struct {
		St   int    `json:"st"`
		SID  int    `json:"sid"`
		TT   []byte `json:"tt"`
		Gr   int    `json:"gr"`
		UUID []byte `json:"uuid"`
	}
	v := new(Value)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := v.Parse(data)
		if err != nil {
			b.Error(err)
		}
		dst.St = parseInt(v.Property("st").Value)
		dst.SID = parseInt(v.Property("sid").Value)
		dst.TT = v.Property("tt").Value
		dst.Gr = parseInt(v.Property("gr").Value)
		dst.UUID = v.Property("uuid").Value
		v.Recycle()
		v.Reset()
	}
}

func parseInt(b []byte) int {
	res, err := strconv.Atoi(b2s(b))
	if err != nil {
		panic("invalid int")
	}
	return res
}

var smallFixture = []byte(`{
    "st": 1,
    "sid": 486,
    "tt": "active",
    "gr": 0,
    "uuid": "de305d54-75b4-431b-adb2-eb6b9e546014",
    "ip": "127.0.0.1",
    "ua": "user_agent",
    "tz": -6,
    "v": 1
}`)

type smallPayload struct {
	UUID string `json:"uuid"`
	Tz   int    `json:"tz"`
	Ua   string `json:"ua"`
	St   int    `json:"st"`
}

func nothing(_ ...interface{}) {}

func BenchmarkJSONParserSuite(b *testing.B) {
	opts := &UnmarshalOptions{
		DisableDataCopy: true,
	}
	b.Run("Small", func(b *testing.B) {
		var dst smallPayload
		b.SetBytes(int64(len(smallFixture)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			opts.Unmarshal(smallFixture, &dst)
			nothing(dst.UUID, dst.Tz, dst.Ua, dst.St)
		}
	})
}

func BenchmarkJSONParserSuiteByJSONParser(b *testing.B) {
	b.Run("Small", func(b *testing.B) {
		uuidKey, tzKey, uaKey, stKey := []byte("uuid"), []byte("tz"), []byte("ua"), []byte("st")
		errStop := errors.New("stop")
		b.SetBytes(int64(len(smallFixture)))
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			var data smallPayload

			missing := 4

			jsonparser.ObjectEach(smallFixture, func(key, value []byte, vt jsonparser.ValueType, off int) error {
				switch {
				case bytes.Equal(key, uuidKey):
					data.UUID, _ = jsonparser.ParseString(value)
					missing--
				case bytes.Equal(key, tzKey):
					v, _ := jsonparser.ParseInt(value)
					data.Tz = int(v)
					missing--
				case bytes.Equal(key, uaKey):
					data.Ua, _ = jsonparser.ParseString(value)
					missing--
				case bytes.Equal(key, stKey):
					v, _ := jsonparser.ParseInt(value)
					data.St = int(v)
					missing--
				}

				if missing == 0 {
					return errStop
				}
				return nil
			})

			nothing(data.UUID, data.Tz, data.Ua, data.St)
		}
	})
}
