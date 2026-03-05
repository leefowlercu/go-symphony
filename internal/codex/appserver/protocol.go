package appserver

import (
	"encoding/json"
	"fmt"
	"strings"
)

type message struct {
	ID     any            `json:"id,omitempty"`
	Method string         `json:"method,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Result map[string]any `json:"result,omitempty"`
	Error  map[string]any `json:"error,omitempty"`
}

func parseLine(line string) (message, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return message{}, fmt.Errorf("empty message")
	}
	var msg message
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return message{}, err
	}
	return msg, nil
}
