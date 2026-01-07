# Personal MCP starter kit

## Motivation

AI tooling is evolving rapidly, with vendors each adding their own methods of
"configuring" their models - various different markdown files like `CLAUDE.md`,
`GEMINI.md` and the slightly more generic `AGENTS.md`. These methods work well
for consistent setup across multiple developers, for a single project. This repo
explores the idea of encapsulating your individual workflow as a developer as a
set of MCP utilities (mostly prompts), in a way that is portable between AI
tools.

## Overview

This repo contains a Go MCP server that includes all prompts in the `prompts/`
directory, embedded in the binary itself - no need to copy around other files!

Prompts require a bit of YAML frontmatter, a `name` attribute at a minimum, and
the remainder of the document is used as the prompt text. There are a few
examples provided you can use for inspiration.

Because the prompts are embedded in the binary, updating prompts requires you to
rebuild the binary. This also means that your prompts can be stored in version
control, and you can determine the version of the prompts from the build data
included in the binary.

## Usage

1. Clone this repo

1. Add or edit files in `prompts` to suit your use case

1. run `go build .` - you now have an MCP server

1. Configure your AI tools to access this MCP server. Configs vary, but it might look something like this:

    ```
    "mcpServers": {
        "personal": {
            "command": "/path/to/your/mcp-starter"
        }
    }
    ```

Your prompts are now available in your AI tool of choice. They can be shared across multiple tools, too!

### Why MCP?

With so many available configuration methods, why did I choose MCP?

For me, the major reason is flexibility. AI tools are changing rapidly, with a
new tool or model emerging every week. To remain agile and avoid lock-in, I want
to be able to use the same tools in any AI system. Today, the "greatest common
factor" among AI tools is MCP. The protocol may not be ideal, but it is
supported everywhere, and is one of the few standards in a rapidly evolving
space.