# Gemini CLI — adapter notes

- Instructions: [`AGENTS.md`](../AGENTS.md) via `context.fileName` in `settings.json`
  (plus thin [`GEMINI.md`](../GEMINI.md) pointer).
- MCP: Gemini requires `mcpServers` inside `settings.json`. Treat repo-root
  [`.mcp.json`](../.mcp.json) as the source of truth — when you change MCP
  servers, update `.mcp.json` first, then mirror into `settings.json`.
- Skills: [`.agents/skills/`](../.agents/skills/) is canonical.
