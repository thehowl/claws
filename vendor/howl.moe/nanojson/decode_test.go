package nanojson

import (
	"io/ioutil"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"
	"unsafe"
)

type fataller interface {
	Fatalf(format string, values ...interface{})
}

func equal(f fataller, got, want interface{}) {
	if !reflect.DeepEqual(want, got) {
		if w, ok := want.([]byte); ok {
			want = string(w)
		}
		if g, ok := got.([]byte); ok {
			got = string(g)
		}
		f.Fatalf("want %v got %v", want, got)
	}
}

func TestSizeOfValue(t *testing.T) {
	t.Log(unsafe.Sizeof(Value{}))
}

func TestValue_Property(t *testing.T) {
	t.Run("NotAnObject", func(t *testing.T) {
		equal(t, new(Value).Property("a"), (*Value)(nil))
	})
	v1 := &Value{
		Kind: KindObject,
		Children: []Value{
			{Kind: KindTrue, Key: []byte("fofo")},
			{Kind: KindNull, Key: []byte("sasageyo")},
		},
		propertyMap: map[string]int{
			"fofo": 0,
			"fifi": 5,
		},
	}
	t.Run("WithPropMap", func(t *testing.T) {
		equal(t, v1.Property("fofo"), &v1.Children[0])
		// will return nil because sasageyo is not in propertyMap
		equal(t, v1.Property("sasageyo"), (*Value)(nil))
		equal(t, v1.Property("fifi"), (*Value)(nil))
		equal(t, v1.Property("haha"), (*Value)(nil))
	})
	t.Run("WithoutPropMap", func(t *testing.T) {
		v1.propertyMap = nil
		equal(t, v1.Property("fofo"), &v1.Children[0])
		equal(t, v1.Property("sasageyo"), &v1.Children[1])
		equal(t, v1.Property("fifi"), (*Value)(nil))
		equal(t, v1.Property("haha"), (*Value)(nil))
	})
}

func TestParseTokens(t *testing.T) {
	table := []struct {
		in         string
		expectKind uint8
	}{
		{"true", KindTrue},
		{"false", KindFalse},
		{"null", KindNull},
		{"true ", KindTrue},
		{" true \r\r\n\t\r", KindTrue},
		{" true", KindTrue},
	}
	for _, tst := range table {
		val := new(Value)
		err := val.Parse([]byte(tst.in))
		if err != nil {
			t.Error(err)
			continue
		}
		equal(t, val.Kind, tst.expectKind)
	}
}

func TestParseString(t *testing.T) {
	table := []struct {
		in   string
		want string
	}{
		{`"meme"`, "meme"},
		{`"\t"`, "\t"},
		{`"\\"`, `\`},
		{`"\/"`, `/`},
		{`"/"`, `/`},
		{`"\u002f"`, `/`},
		{`"\u002F"`, `/`},
		{`"\u4aaa"`, `䪪`},
		{`"\u4aaaowo"`, `䪪owo`},
		{`"\uD834"`, string(unicode.ReplacementChar)},
		{`"\uD834\uDD1E"`, "𝄞"},
		{`"𝄞"`, "𝄞"},
		{`"test\uD834\uDD1e"`, "test𝄞"},
	}
	for _, tst := range table {
		val := new(Value)
		in := []byte(tst.in)
		err := val.Parse(in)
		if err != nil {
			t.Error(tst.in, err)
			continue
		}
		equal(t, val.Kind, KindString)
		equal(t, string(val.Value), tst.want)
	}
}

func TestNewChild(t *testing.T) {
	tt := []struct{ runs, expectCap int }{
		{0, 0},
		{1, 1024},
		{1, 1024},
		{8, 1024},
		{1024, 1024},
	}
	for _, x := range tt {
		v := new(Value)
		start := time.Now()
		for i := 0; i < x.runs; i++ {
			v.newChild()
		}
		t.Logf("creating %d children takes %v", x.runs, time.Since(start))
		equal(t, cap(v.Children), x.expectCap)
		v.Recycle()
	}
}

func TestParseArray(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte("[]"))
		equal(t, err, nil)
		equal(t, v.Kind, KindArray)
		equal(t, v.Children, ([]Value)(nil))
	})
	t.Run("OneNumber", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte("[     \n\n\t12            ]"))
		equal(t, err, nil)
		equal(t, len(v.Children), 1)
		equal(t, v.Children[0].Kind, KindNumber)
		equal(t, v.Children[0].Value, []byte("12"))
	})
	t.Run("TwoNumbers", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte(" [ 12\n,\n   21]"))
		equal(t, err, nil)
		equal(t, len(v.Children), 2)
		equal(t, v.Children[0].Kind, KindNumber)
		equal(t, v.Children[0].Value, []byte("12"))
		equal(t, v.Children[1].Kind, KindNumber)
		equal(t, v.Children[1].Value, []byte("21"))
	})
	t.Run("Nested", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte("[[], [1337]]"))
		equal(t, err, nil)
		equal(t, len(v.Children), 2)
		equal(t, v.Children[0].Kind, KindArray)
		equal(t, v.Children[1].Kind, KindArray)
		equal(t, len(v.Children[0].Children), 0)
		equal(t, len(v.Children[1].Children), 1)
		equal(t, v.Children[1].Children[0].Kind, KindNumber)
		equal(t, v.Children[1].Children[0].Value, []byte("1337"))
	})
}

func TestParseObject(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte("{}"))
		equal(t, err, nil)
		equal(t, v.Kind, KindObject)
		equal(t, v.Children, ([]Value)(nil))
	})
	t.Run("OneKV", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte(`{"key":"value"}`))
		equal(t, err, nil)
		equal(t, v.Kind, KindObject)
		equal(t, len(v.Children), 1)
		equal(t, v.Children[0].Kind, KindString)
		equal(t, v.Children[0].Key, []byte("key"))
		equal(t, v.Children[0].Value, []byte("value"))
	})
	t.Run("TwoKV", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte(`{ "key"  :  "value", "key2": 1337 }`))
		equal(t, err, nil)
		equal(t, v.Kind, KindObject)
		equal(t, len(v.Children), 2)

		equal(t, v.Children[0].Kind, KindString)
		equal(t, v.Children[0].Key, []byte("key"))
		equal(t, v.Children[0].Value, []byte("value"))

		equal(t, v.Children[1].Kind, KindNumber)
		equal(t, v.Children[1].Key, []byte("key2"))
		equal(t, v.Children[1].Value, []byte("1337"))

		equal(t, v.Property("No"), (*Value)(nil))
		equal(t, v.Property("key"), &v.Children[0])
		equal(t, v.Property("key2"), &v.Children[1])
	})
	t.Run("Nested", func(t *testing.T) {
		v := new(Value)
		err := v.Parse([]byte(`{"w": {"k":"v"}}`))
		equal(t, err, nil)
		equal(t, v.Kind, KindObject)
		equal(t, len(v.Children), 1)

		equal(t, v.Children[0].Kind, KindObject)
		equal(t, v.Children[0].Key, []byte("w"))
		equal(t, len(v.Children[0].Children), 1)

		equal(t, v.Children[0].Children[0].Kind, KindString)
		equal(t, v.Children[0].Children[0].Key, []byte("k"))
		equal(t, v.Children[0].Children[0].Value, []byte("v"))
	})
}

func TestParseNumber(t *testing.T) {
	table := []struct {
		in   string
		kind uint8
	}{
		{`0`, 255},
		{`1`, 255},
		{`-`, KindNumber},
		{`3.14`, 255},
		{`3.`, KindNumber},
		{`0.no`, KindNumber},
		{`-Haha`, KindNumber},
		{`0e5`, 255},
		{`1e5`, 255},
		{`1E5`, 255},
		{`15.41e0`, 255},
		{`15.41e161`, 255},
		{`1e`, KindNumber},
		{`1e+1`, 255},
		{`1e-1`, 255},
		{`1e--`, KindNumber},
		{`1e-`, KindNumber},
		{`potato`, KindInvalid},
	}
	for _, tst := range table {
		val := new(Value)
		in := []byte(tst.in)
		errRaw := val.Parse(in)
		if tst.kind == 255 {
			equal(t, errRaw, nil)
			continue
		}
		err, ok := errRaw.(*ParseError)
		if !ok {
			t.Errorf("expecting *ParseError, got %T", errRaw)
			continue
		}
		equal(t, err.Kind, tst.kind)
	}
}

func TestParseError(t *testing.T) {
	tt := []struct {
		input string
		pos   int
	}{
		{`"`, 0},
		{`"AA`, 2},
		{`[1337`, 4},
		{`]`, 0},
		{`Fofo`, 0},
		{`fals`, 0},
		{`faLse`, 0},
		{`   `, 2},
		{`"\`, 1},
		{`"\X"`, 2},
		{"\"\x00\"", 1},
		{`"\u`, 2},
		{`"\ux`, 2},
		{`"\uxx`, 2},
		{`"\uxxx`, 2},
		{`"\uxxxx`, 3},
		{`"\uaaa`, 2},
		{`-`, 0},
		{`1.`, 1},
		{`0e`, 1},
		{`0e+`, 2},
		{`-a`, 1},
		{`0.a`, 2},
		{`0e+a`, 3},
		{`[1 1]`, 3},
		{`[1,,1]`, 3},
		{`[1,]`, 3},
		{`{1}`, 1},
		{`{:`, 1},
		{`{,`, 1},
		{`{"k",`, 4},
		{`{"k"}`, 4},
		{`{`, 0},
		{`{"key"`, 5},
		{`{"k":1`, 5},
	}
	for _, tst := range tt {
		v := new(Value)
		errRaw := v.Parse([]byte(tst.input))
		if errRaw == nil {
			t.Errorf("%q: nil", tst.input)
			continue
		}
		err, ok := errRaw.(*ParseError)
		if !ok {
			t.Errorf("%q: want ParseError got %T", tst.input, errRaw)
			continue
		}
		t.Logf("%q %v", tst.input, errRaw)
		equal(t, tst.pos, err.Pos)
		equal(t, string(tst.input[err.Pos]), string(err.Char))
	}
}

func TestEmpty(t *testing.T) {
	err := new(Value).Parse([]byte{})
	equal(t, err, errBIsEmpty)
}

func BenchmarkParseToken(b *testing.B) {
	f := []byte("true")
	v := new(Value)
	b.SetBytes(int64(len(f)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Parse(f)
	}
}

func BenchmarkParseString(b *testing.B) {
	b.Run("UnicodeEscape", func(b *testing.B) {
		v := new(Value)
		b1 := []byte(`"test\uD834\uDD1e"`)
		b2 := []byte(`"test\uD834\uDD1e"`)
		b.SetBytes(int64(len(b1)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v.Parse(b2)
			copy(b2, b1)
		}
	})
	b.Run("ASCII", func(b *testing.B) {
		v := new(Value)
		b1 := []byte(`"testtesttest"`)
		b2 := []byte(`"testtesttest"`)
		b.SetBytes(int64(len(b2)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v.Parse(b2)
			copy(b2, b1)
		}
	})
}

func TestSuite(t *testing.T) {
	files, err := ioutil.ReadDir("testdata")
	equal(t, err, nil)
	for _, file := range files {
		b, err := ioutil.ReadFile("testdata/" + file.Name())
		equal(t, err, nil)
		v := new(Value)
		err = v.Parse(b)
		wantError := strings.HasPrefix(file.Name(), "fail")
		if (err == nil) == wantError {
			t.Error(file.Name(), "want", wantError, "opposite happened instead")
			continue
		}
	}
}

func BenchmarkValidateNumber(b *testing.B) {
	v := new(Value)
	f := []byte("13377331")
	b.SetBytes(int64(len(f)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		v.Parse(f)
	}
}

func BenchmarkParseArray(b *testing.B) {
	b.Run("None", func(b *testing.B) {
		v := new(Value)
		f := []byte("[]")
		b.SetBytes(int64(len(f)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v.Parse(f)
			v.Recycle()
		}
	})
	b.Run("Three", func(b *testing.B) {
		v := new(Value)
		f := []byte("[1, 2, 3]")
		b.SetBytes(int64(len(f)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v.Parse(f)
			v.Recycle()
		}
	})
	b.Run("Eight", func(b *testing.B) {
		v := new(Value)
		f := []byte("[1, 2, 3, 4, 5, 6, 7, 8]")
		b.SetBytes(int64(len(f)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v.Parse(f)
			v.Recycle()
		}
	})
}

func BenchmarkParseObject(b *testing.B) {
	b.Run("SmallObject", func(b *testing.B) {
		v := new(Value)
		s := []byte(`{"key": "value", "key2": 1337}`)
		b.SetBytes(int64(len(s)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			v.Parse(s)
			v.Recycle()
		}
	})
	b.Run("Medium", func(b *testing.B) {
		v := new(Value)
		s := []byte(`{
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
		b.SetBytes(int64(len(s)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := v.Parse(s)
			if err != nil {
				b.Error(err)
			}
			v.Recycle()
		}
	})
	b.Run("Large", func(b *testing.B) {
		v := new(Value)
		s := []byte(`{
	"code": 200,
	"id": 1009,
	"username": "Howl",
	"username_aka": "",
	"registered_on": "2016-01-12T06:57:57+01:00",
	"privileges": 7339775,
	"latest_activity": "2017-12-16T20:46:59+01:00",
	"country": "IT",
	"std": {
		"ranked_score": 1064522013,
		"total_score": 3528005871,
		"playcount": 2342,
		"replays_watched": 13,
		"total_hits": 151495,
		"level": 81.1339717773551,
		"accuracy": 98.048836,
		"pp": 2493,
		"global_leaderboard_rank": 2554,
		"country_leaderboard_rank": 41
	},
	"taiko": {
		"ranked_score": 518546,
		"total_score": 1073293,
		"playcount": 17,
		"replays_watched": 1,
		"total_hits": 1348,
		"level": 5.678714545454546,
		"accuracy": 84.134605,
		"pp": 97,
		"global_leaderboard_rank": 784,
		"country_leaderboard_rank": 20
	},
	"ctb": {
		"ranked_score": 48625556,
		"total_score": 432660532,
		"playcount": 108,
		"replays_watched": 1,
		"total_hits": 38041,
		"level": 40.433967037037036,
		"accuracy": 98.170616,
		"pp": 0,
		"global_leaderboard_rank": 517,
		"country_leaderboard_rank": 10
	},
	"mania": {
		"ranked_score": 2418144,
		"total_score": 5146838,
		"playcount": 26,
		"replays_watched": 0,
		"total_hits": 984,
		"level": 9.413355555555556,
		"accuracy": 78.161026,
		"pp": 7,
		"global_leaderboard_rank": 2920,
		"country_leaderboard_rank": 52
	},
	"play_style": 0,
	"favourite_mode": 0,
	"badges": [
		{
			"id": 2,
			"name": "Developer",
			"icon": "blue fa-code"
		},
		{
			"id": 31,
			"name": "Translator",
			"icon": "fa-globe"
		},
		{
			"id": 14,
			"name": "Donator",
			"icon": "fa-money"
		}
	],
	"custom_badge": {
		"name": "Illuminati",
		"icon": "eye inverted red"
	},
	"silence_info": {
		"reason": "",
		"end": "1970-01-01T01:00:00+01:00"
	}
}`)
		b.SetBytes(int64(len(s)))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := v.Parse(s)
			if err != nil {
				b.Error(err)
			}
			v.Recycle()
		}
	})
}
