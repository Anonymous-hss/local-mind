package main

import (
	"encoding/json"
	"fmt"
	"github.com/localmind/core/pkg/protocol"
)

func main() {
	pong := protocol.PongPayload{
		Version:      "0.0.1",
		OllamaStatus: protocol.OllamaConnected,
		Models: []protocol.ModelInfo{
			{Name: "qwen2.5-coder:1.5b", Size: 1234, Role: "completion"},
		},
		ActiveModels: map[string]string{"completion": "qwen2.5-coder:1.5b"},
	}
	b, _ := json.MarshalIndent(pong, "", "  ")
	fmt.Println(string(b))

    resp := protocol.Response{
        Type: protocol.ResponseTypePong,
        Payload: b,
    }
    b2, _ := json.MarshalIndent(resp, "", "  ")
    fmt.Println(string(b2))
}
