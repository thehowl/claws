package main

import "encoding/json"

func attemptJSONFormatting(msg string) string {
	var i interface{}
	err := json.Unmarshal([]byte(msg), &i)
	if err != nil {
		return msg
	}
	data, err := json.MarshalIndent(i, "", "\t")
	if err != nil {
		return msg
	}
	return string(data)
}
