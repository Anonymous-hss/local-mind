// ipc_smoke_test.go — manual IPC smoke test for the LocalMind core binary.
// Run with: go run ipc_smoke_test.go
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// Minimal IPC message structs mirroring the protocol
type Request struct {
	RequestID string      `json:"requestId"`
	Type      string      `json:"type"`
	Payload   interface{} `json:"payload,omitempty"`
}

type Response struct {
	RequestID string          `json:"requestId"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     string          `json:"error,omitempty"`
}

func writeMsg(w io.Writer, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))
	if _, err := w.Write(lenBuf); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func readMsg(r io.Reader) (*Response, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}
	size := binary.BigEndian.Uint32(lenBuf)
	if size > 10*1024*1024 {
		return nil, fmt.Errorf("message too large: %d bytes", size)
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func main() {
	binaryPath := "./localmind-core.exe"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		binaryPath = "./localmind-core"
	}

	fmt.Printf("▶ Starting core binary: %s\n", binaryPath)
	cmd := exec.Command(binaryPath)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Printf("❌ StdinPipe: %v\n", err)
		os.Exit(1)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("❌ StdoutPipe: %v\n", err)
		os.Exit(1)
	}

	if err := cmd.Start(); err != nil {
		fmt.Printf("❌ Failed to start binary: %v\n", err)
		os.Exit(1)
	}
	defer cmd.Process.Kill()

	// Give the process a moment to initialise
	time.Sleep(200 * time.Millisecond)

	tests := []struct {
		name    string
		request Request
	}{
		{
			name: "Handshake",
			request: Request{
				RequestID: "req-1",
				Type:      "handshake",
				Payload:   map[string]string{"extensionVersion": "0.1.0"},
			},
		},
		{
			name: "Ping",
			request: Request{
				RequestID: "req-2",
				Type:      "ping",
			},
		},
	}

	passed := 0
	for _, tt := range tests {
		fmt.Printf("\n  ▷ %s (id=%s)...\n", tt.name, tt.request.RequestID)

		if err := writeMsg(stdin, tt.request); err != nil {
			fmt.Printf("  ❌ Write failed: %v\n", err)
			continue
		}

		// Read response with timeout via channel
		type result struct {
			resp *Response
			err  error
		}
		ch := make(chan result, 1)
		go func() {
			r, e := readMsg(stdout)
			ch <- result{r, e}
		}()

		select {
		case res := <-ch:
			if res.err != nil {
				fmt.Printf("  ❌ Read failed: %v\n", res.err)
			} else if res.resp.Error != "" {
				fmt.Printf("  ❌ Core error: %s\n", res.resp.Error)
			} else {
				fmt.Printf("  ✅ OK  type=%s payload=%s\n", res.resp.Type, string(res.resp.Payload))
				passed++
			}
		case <-time.After(3 * time.Second):
			fmt.Printf("  ❌ Timeout waiting for response\n")
		}
	}

	fmt.Printf("\n══════════════════════════════\n")
	fmt.Printf("Results: %d/%d passed\n", passed, len(tests))
	if passed == len(tests) {
		fmt.Println("🎉 Core engine IPC: ALL PASS")
		os.Exit(0)
	} else {
		fmt.Println("💥 Some tests FAILED")
		os.Exit(1)
	}
}
