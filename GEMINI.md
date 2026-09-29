# Gemini CLI — project instructions

Canonical agent rules live in [`AGENTS.md`](AGENTS.md). Read and follow that
file; do not duplicate its rules here.

MCP servers are declared in [`.mcp.json`](.mcp.json). Project Gemini settings
(`.gemini/settings.json`) point `context.fileName` at `AGENTS.md` and mirror
those MCP servers for the CLI — edit `.mcp.json` first, then keep the Gemini
`mcpServers` block in sync.

Skills: [`.agents/skills/`](.agents/skills/) is canonical.
