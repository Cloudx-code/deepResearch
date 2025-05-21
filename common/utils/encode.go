package utils

import "encoding/json"

func Encode(i interface{}) string {
	s, _ := json.Marshal(i)
	return string(s)
}
