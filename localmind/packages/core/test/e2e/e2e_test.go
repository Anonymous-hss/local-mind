package e2e

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/localmind/core/pkg/protocol"
)

func TestE2E_Binary(t *testing.T) {
	// 1. Build the binary
	tmpDir, err := os.MkdirTemp("", "localmind-e2e")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	exePath := filepath.Join(tmpDir, "localmind.exe")
	if os.PathSeparator == '/' {
		exePath = filepath.Join(tmpDir, "localmind")
	}

	cmdBuild := exec.Command("go", "build", "-tags", "nocgo", "-o", exePath, "../../cmd/localmind")
	if out, err := cmdBuild.CombinedOutput(); err != nil {
		t.Fatalf("Build failed: %v\nOutput: %s", err, out)
	}

	// 2. Start the binary
	cmd := exec.Command(exePath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to get stdin: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("Failed to get stdout: %v", err)
	}

	// Capture stderr for debugging
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start binary: %v", err)
	}
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	// Helper to send request
	sendRequest := func(req *protocol.Request) {
		jsonData, _ := json.Marshal(req)
		length := uint32(len(jsonData))

		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, length)

		if _, err := stdin.Write(lenBuf); err != nil {
			t.Fatalf("Failed to write length: %v", err)
		}
		if _, err := stdin.Write(jsonData); err != nil {
			t.Fatalf("Failed to write payload: %v", err)
		}
	}

	// Helper to read response
	readResponse := func() *protocol.Response {
		reader := bufio.NewReader(stdout)

		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(reader, lenBuf); err != nil {
			t.Fatalf("Failed to read response length: %v", err)
		}

		length := binary.BigEndian.Uint32(lenBuf)
		data := make([]byte, length)
		if _, err := io.ReadFull(reader, data); err != nil {
			t.Fatalf("Failed to read response data: %v", err)
		}

		var resp protocol.Response
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}
		return &resp
	}

	// 3. Test Handshake
	t.Log("Testing Handshake...")
	handshakeReq := &protocol.Request{
		ID:        "e2e-handshake",
		Timestamp: time.Now().Unix(),
		Type:      protocol.RequestTypeHandshake,
	}
	// Payload needed? Yes, protocol version.
	handshakePayload := protocol.HandshakePayload{
		ProtocolVersion:  "1.0.0",
		ExtensionVersion: "0.0.1-e2e",
	}
	payloadBytes, _ := json.Marshal(handshakePayload)
	handshakeReq.Payload = payloadBytes

	sendRequest(handshakeReq)

	resp := readResponse()
	if resp.Type != protocol.ResponseTypeHandshakeAck {
		t.Errorf("Handshake failed, got type: %s", resp.Type)
	}

	// 4. Test Ping (checks Ollama connection)
	t.Log("Testing Ping...")
	pingReq := &protocol.Request{
		ID:        "e2e-ping",
		Timestamp: time.Now().Unix(),
		Type:      protocol.RequestTypePing,
	}
	sendRequest(pingReq)

	resp = readResponse()
	if resp.Type != protocol.ResponseTypePong {
		t.Errorf("Ping failed, got type: %s", resp.Type)
	}

	var pong protocol.PongPayload
	json.Unmarshal(resp.Payload, &pong)
	t.Logf("Ollama Status: %s", pong.OllamaStatus)

	// 5. Test Completion (Real Model)
	t.Log("Testing Completion...")
	compReq := &protocol.Request{
		ID:        "e2e-comp",
		Timestamp: time.Now().Unix(),
		Type:      protocol.RequestTypeCompletion,
	}
	compPayload := protocol.CompletionPayload{
		Prefix:    "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Printl",
		Language:  "go",
		MaxTokens: 10,
	}
	pBytes, _ := json.Marshal(compPayload)
	compReq.Payload = pBytes

	sendRequest(compReq)

	// Set deadline for completion
	done := make(chan struct{})
	go func() {
		resp = readResponse()
		close(done)
	}()

	select {
	case <-done:
		if resp.Type != protocol.ResponseTypeResult {
			t.Errorf("Completion failed, got type: %s", resp.Type)
		}
		var res protocol.ResultPayload
		json.Unmarshal(resp.Payload, &res)
		t.Logf("Completion: %q", res.Content)
		if res.Content == "" {
			t.Error("Completion returned empty content")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Completion timed out")
	}
}
