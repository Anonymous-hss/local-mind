package main

import (
    "context"
    "fmt"
    "github.com/localmind/core/pkg/ollama"
)

func main() {
    client := ollama.NewClient(ollama.DefaultClientConfig())
    models, err := client.ListModels(context.Background())
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    fmt.Printf("Models found: %d\n", len(models))
    for _, m := range models {
        fmt.Printf("- %s\n", m.Name)
    }
}
