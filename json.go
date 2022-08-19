package main

import (
	"bytes"

	"howl.moe/nanojson"
)

func attemptJSONFormatting(msg []byte) []byte {
	virtualV := nanojson.Pools.Value.Get()
	v := virtualV.(*nanojson.Value)
	err := v.Parse(msg)
	if err != nil {
		return msg
	}
	buf := new(bytes.Buffer)
	printValue(buf, v, "")
	return buf.Bytes()
}

func printValue(buf *bytes.Buffer, v *nanojson.Value, indent string) {
	indent += "  "
	switch v.Kind {
	case nanojson.KindString:
		v.EncodeJSON(buf)
	case nanojson.KindNumber:
		buf.Write(v.Value)
	case nanojson.KindObject:
		if len(v.Children) == 0 {
			buf.WriteString("{}")
			return
		}
		// get tmpV which we will use to encode keys as JSON strings
		virtualTmpV := nanojson.Pools.Value.Get()
		tmpV := virtualTmpV.(*nanojson.Value)
		tmpV.Kind = nanojson.KindString
		if len(v.Children) == 1 {
			buf.WriteByte('{')
			// encode key
			tmpV.Value = v.Children[0].Key
			tmpV.EncodeJSON(buf)
			buf.WriteString(": ")
			// encode value
			printValue(buf, &v.Children[0], indent)
			buf.WriteByte('}')
			return
		}
		buf.WriteString("{\n")
		for i := 0; i < len(v.Children); i++ {
			buf.WriteString(indent)
			tmpV.Value = v.Children[i].Key
			tmpV.EncodeJSON(buf)
			buf.WriteString(": ")
			printValue(buf, &v.Children[i], indent)
			if i != len(v.Children)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(indent[:len(indent)-2])
		buf.WriteByte('}')
	case nanojson.KindArray:
		switch len(v.Children) {
		case 0:
			buf.WriteString("[]")
		case 1:
			buf.WriteByte('[')
			printValue(buf, &v.Children[0], indent)
			buf.WriteByte(']')
		default:
			buf.WriteString("[\n")
			for i := 0; i < len(v.Children); i++ {
				buf.WriteString(indent)
				printValue(buf, &v.Children[i], indent)
				if i != len(v.Children)-1 {
					buf.WriteByte(',')
				}
				buf.WriteByte('\n')
			}
			buf.WriteString(indent[:len(indent)-2])
			buf.WriteByte(']')
		}
	case nanojson.KindTrue:
		buf.WriteString("true")
	case nanojson.KindFalse:
		buf.WriteString("false")
	case nanojson.KindNull:
		buf.WriteString("null")
	default:
		buf.WriteString("(INVALID)")
	}
}
