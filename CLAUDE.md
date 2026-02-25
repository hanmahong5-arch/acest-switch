# Acest Switch (legacy → migrating to lurus-switch)

Desktop AI CLI gateway + admin backend. Wails 3 + Go + Vue 3.

## Commands

```bash
# Desktop app
wails3 task dev                    # Dev with hot reload
wails3 task build                  # Build
go test ./...                      # Backend tests
cd frontend && bun run build       # Frontend only

# Sync service (独立后端)
cd sync-service && go run cmd/main.go    # Starts on :8081
```

## Architecture

```
Gateway (:18100) → Provider Select (round-robin/priority/failover) → AI Providers
                 → NEW-API mode (optional, unified gateway)
                 → NATS sync (optional, multi-device)
```

Routes: `/v1/messages` (Claude) / `/responses` (Codex) / `/v1/chat/completions` (Codex/OpenAI) / `/pc/v1/chat/completions` (PicoClaw) / `/v1beta/models/*` (Gemini)

Config stored in `~/.code-switch/` (claude-code.json, codex.json, picoclaw.json, mcp.json, app.json, app.db).
PicoClaw config: `~/.picoclaw/config.json` (model_list array with `api_base` pointing to `/pc/v1`).

## Key Backend Services

| Service | Purpose |
|---------|---------|
| ProviderRelayService | HTTP proxy, NEW-API forwarding, Gemini conversion, PicoClaw `/pc/` routing |
| ProviderService | Provider CRUD, model validation |
| LogService | Request logging (SQLite write queue for perf) |
| SyncService | NATS client, multi-device sync |
| MCPService / SkillService | MCP + skill management |

## Key Technical Details

- **SQLite write queue**: single-goroutine batch writes (10 records / 100ms) to avoid lock contention
- **Price pre-calculation**: costs computed at insert time, not query time
- **Model matching**: exact, wildcard (`claude-*`), and mapping (`claude-*` → `anthropic/claude-*`)
- See `doc/` for detailed architecture, DB schema, and troubleshooting

## BMAD

| Resource | Path |
|----------|------|
| Architecture | `./_bmad-output/planning-artifacts/architecture.md` |
