package nanojson

import (
	"bytes"
	"testing"
)

var fullTests = [...]string{
	`[1,2,3,1.33e4,true,null,"1337çé7331",{"key":13,"key2":false}]`,
	`null`,
	`7.14`,
	`""`,
	`"test"`,
	`"\u0008"`,
}

func TestFull(t *testing.T) {
	buf := new(bytes.Buffer)
	for _, f := range fullTests {
		buf.Reset()
		v := new(Value)
		err := v.Parse([]byte(f))
		if err != nil {
			t.Error(f, err)
			continue
		}
		_, err = v.EncodeJSON(buf)
		if err != nil {
			t.Error(f, err)
			continue
		}
		got := string(buf.Bytes())
		if got != f {
			t.Errorf("got %q want %q", got, f)
		}
	}
}

const encBenchmark = `{
    "st": 1,
    "sid": 486,
    "tt": "active",
    "gr": 0,
    "uuid": ["de305d54-75b4-431b-adb2-eb6b9e546014"],
    "ip": "127.0.0.1",
    "ua": "user_agent",
    "tz": -6,
    "v": 1
}`

func BenchmarkEncode(b *testing.B) {
	buf := new(bytes.Buffer)
	v := new(Value)
	v.Parse([]byte(encBenchmark))
	written, err := v.EncodeJSON(buf)
	if err != nil {
		b.Error(err)
	}
	b.SetBytes(int64(written))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.EncodeJSON(buf)
		buf.Reset()
	}
}
