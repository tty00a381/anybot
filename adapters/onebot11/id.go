package onebot11

import (
	"encoding/json"
	"strconv"
	"strings"
)

type flexibleInt64 int64

func (id *flexibleInt64) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "" || text == "null" {
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		text = strings.TrimSpace(value)
		if text == "" {
			return nil
		}
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*id = flexibleInt64(parsed)
	return nil
}

func (id flexibleInt64) int64() int64 {
	return int64(id)
}
