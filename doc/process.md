# CodeSwitch Process Log

> Previous entries archived to `doc/archive/process_v20260116.md`

---

## 2026-02-25: PicoClaw Integration — 4th CLI Platform
Added PicoClaw as the 4th AI CLI platform (alongside Claude Code, Codex, Gemini CLI).
Route strategy: `/pc/v1/chat/completions` prefix for platform disambiguation from Codex's `/v1/chat/completions`.
Verification: `go build ./...` PASS, `go test -run PicoClaw|DetectApp|NormalizeAppName` 7/7 PASS, `bun run build` PASS.
Remaining: `providerservice_v2_test.go` has pre-existing compile errors (unrelated).
