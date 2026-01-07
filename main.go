package main

import (
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"

	"github.com/adrg/frontmatter"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

//go:embed prompts/*.md
var promptfs embed.FS

func main() {
	server := mcp.NewServer(&mcp.Implementation{Name: "personal-mcp"}, nil)

	// Add prompts from the embedded prompt filesystem.
	matches, err := fs.Glob(promptfs, "prompts/*.md")
	log.Print(matches)
	if err != nil {
		log.Fatal(err)
	}
	for _, fn := range matches {
		log.Printf("adding prompt from %s", fn)
		f, err := promptfs.Open(fn)
		if err != nil {
			log.Printf("failed to read file %s: %v", fn, err)
		}
		p, ph, err := fromMarkdown(f)
		if err != nil {
			log.Printf("failed to parse prompt file %s: %v", fn, err)
		}
		server.AddPrompt(&p, ph)
		log.Printf("added %#v", p)

	}

	// Run the server on the stdio transport.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}

type PromptMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func fromMarkdown(f io.Reader) (mcp.Prompt, mcp.PromptHandler, error) {
	var meta PromptMetadata
	content, err := frontmatter.Parse(f, &meta)
	if err != nil {
		return mcp.Prompt{}, nil, fmt.Errorf("error reading prompt: %w", err)
	}
	if meta.Name == "" {
		return mcp.Prompt{}, nil, fmt.Errorf("no metadata found. skipping.")
	}
	var ph mcp.PromptHandler = func(ctx context.Context, gpr *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {

		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{
				{
					Role:    "assistant",
					Content: &mcp.TextContent{Text: string(content)},
				},
			},
		}, nil
	}
	return mcp.Prompt{Name: meta.Name, Description: meta.Description}, ph, nil
}
