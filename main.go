// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path"
	"runtime/debug"

	"github.com/adrg/frontmatter"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/muncus/mcp-starter/auth"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v3"
)

//go:embed prompts/*.md
var promptfs embed.FS

var binaryName = path.Base(os.Args[0])

func main() {

	cfgdir, err := os.UserConfigDir()
	if err != nil {
		log.Fatalf("could not find user config dir. check $XDG_CONFIG_DIR. %v", err)
	}
	cfgpath := path.Join(cfgdir, binaryName, "mcpconfig.yaml")
	viper.SetConfigFile(cfgpath)

	err = viper.ReadInConfig()
	if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
		log.Printf("no config found at %s. using defaults\n", cfgpath)
	} else if err != nil {
		log.Fatalf("error reading config: %v", err)
	}

	cmd := &cli.Command{
		Name:  binaryName,
		Usage: "A personal MCP server.",
		Action: func(ctx context.Context, c *cli.Command) error {
			return serve()
		},
		Commands: []*cli.Command{
			{
				Name:  "serve",
				Usage: "Launch the MCP server (default)",
				Action: func(ctx context.Context, c *cli.Command) error {
					return serve()
				},
			},
			auth.Command(),
			&cli.Command{
				Name:  "version",
				Usage: "Print build version",
				Action: func(ctx context.Context, c *cli.Command) error {
					return version()
				},
			},
		},
	}

	cmd.Run(context.Background(), os.Args)

}

func version() error {
	if info, ok := debug.ReadBuildInfo(); ok {
		var revision string
		var modified bool
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" && setting.Value == "true" {
				modified = true
			}
		}
		if revision != "" {
			fmt.Printf("commit: %s", revision)
			if modified {
				fmt.Print(" (dirty)")
			}
			fmt.Println()
		} else {
			fmt.Println("build info not available")
		}
	} else {
		fmt.Println("build info not available")
	}
	return nil
}

func serve() error {
	server := mcp.NewServer(&mcp.Implementation{Name: binaryName}, nil)

	// Add prompts from the embedded prompt filesystem.
	matches, err := fs.Glob(promptfs, "prompts/*.md")
	log.Print(matches)
	if err != nil {
		log.Fatal(err)
	}
	for _, fn := range matches {
		log.Printf("adding prompt from %s", fn)
		err = addFromMarkdown(server, fn)
		if err != nil {
			log.Printf("failed to add file %s: %v", fn, err)
		}
	}

	// Run the server on the stdio transport.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		return err
	}
	return nil
}

type PromptMetadata struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func addFromMarkdown(s *mcp.Server, fname string) error {
	f, err := promptfs.Open(fname)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", fname, err)
	}
	var meta PromptMetadata
	content, err := frontmatter.Parse(f, &meta)
	if err != nil {
		return fmt.Errorf("error reading prompt: %w", err)
	}
	if meta.Name == "" {
		return fmt.Errorf("no metadata found. skipping.")
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
	s.AddPrompt(&mcp.Prompt{Name: meta.Name, Description: meta.Description}, ph)

	// Include Prompts as Resources as well.
	resUri := fmt.Sprintf("embed://%s", fname)
	r := &mcp.Resource{
		Name:        meta.Name,
		Description: meta.Description,
		URI:         resUri,
	}
	rh := func(ctx context.Context, rrr *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Meta: mcp.Meta{},
			Contents: []*mcp.ResourceContents{
				{
					URI:  resUri,
					Text: string(content),
				},
			},
		}, nil

	}
	s.AddResource(r, rh)

	return nil
}
