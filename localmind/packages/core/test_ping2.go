package main

import (
    "context"
    "fmt"
    "github.com/localmind/core/internal/ollama"
)

func main() {
    client := ollama.NewClient(ollama.DefaultClientConfig())
    err := client.Ping(context.Background())
    if err != nil {
        fmt.Printf("Ping failed: %v\n", err)
    } else {
        fmt.Println("Ping succeeded")
    }
}
