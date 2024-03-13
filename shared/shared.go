package shared

import "encoding/json"

type Event struct {
	Event string        `json:"event"`
	Data  []interface{} `json:"data"`
}

func (e *Event) String() string {
	data, _ := json.Marshal(e)
	return string(data)
}
