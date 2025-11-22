package websocket

import (
	"encoding/json"
	"log"
)

// mustMarshal marshals an object to JSON, panics on error
func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("WebSocket: Failed to marshal message: %v", err)
		return []byte("{}")
	}
	return data
}
