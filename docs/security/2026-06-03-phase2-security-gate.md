# Phase 2/3 Security Gate

> Date: 2026-06-03

## Gate Rule

OpenForge should not expand the browser workbench surface until these controls are verified in code and tests.

## Required Controls

| Control | Required behavior | Verification |
|---------|-------------------|--------------|
| JWT location | Browser API calls use `Authorization: Bearer`, not URL query tokens | `frontend/src/shared/api.ts`, route tests |
| WebSocket auth | `/ws/chat` rejects missing or invalid tokens before upgrade | `internal/server/ws_handler_test.go` |
| CSP header | HTTP responses include a restrictive Content-Security-Policy | `internal/server/middleware_test.go` |
| XSS sanitizer | Markdown/HTML rendering uses shared sanitizer | `frontend/src/shared/sanitize.ts` and component review |
| Node.js IO exposure | Node service is not treated as public browser API | deployment and server config review |

## Acceptance Commands

```powershell
$env:GOCACHE='D:\vscode\tiktok\openforge\.cache\go-build'
New-Item -ItemType Directory -Force -Path $env:GOCACHE | Out-Null
go test ./internal/server -run "TestSecurityHeaders|TestChatWS"
cd frontend
npm run typecheck
npm test
```
