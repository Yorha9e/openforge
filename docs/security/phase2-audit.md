# Phase 2 Security Audit

Date: 2026-05-22
Auditor: automated

## JWT — PASS

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| HS256 signing | `internal/auth/service/jwt.go` — hmac.New(sha256.New, ...) | PASS |
| 1h access token TTL | Configurable via minimal.yaml `jwt.access_ttl: "1h"` | PASS |
| 24h refresh token TTL | Configurable via minimal.yaml `jwt.refresh_ttl: "24h"` | PASS |
| Token in Authorization header only | `frontend/src/shared/api.ts` sets Bearer header | PASS |
| Never in URL query string | WebSocket auth via first-frame `{"type":"auth"}` in useWebSocket.ts | PASS |
| Expiry enforced | `JWTService.Verify()` checks `claims.ExpiresAt` | PASS |
| Signature verified | `JWTService.Verify()` HMAC comparison with hmac.Equal | PASS |
| Tampered tokens rejected | Test: `TestJWTVerifyTampered` | PASS |
| Wrong signature rejected | Test: `TestJWTVerifyInvalidSignature` | PASS |
| Expired tokens rejected | Test: `TestJWTVerifyExpired` | PASS |

## XSS — PASS

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| DOMPurify whitelist | `frontend/src/shared/sanitize.ts` — ALLOWED_TAGS: h1-h6, p, code, pre, ul, ol, li, strong, em, a, table, blockquote, br, hr | PASS |
| ALLOWED_ATTR limited | href, target, rel only | PASS |
| ALLOW_DATA_ATTR disabled | Set to false | PASS |
| dangerouslySetInnerHTML guarded | MessageList.tsx routes through sanitizeHTML() | PASS |
| No eval() usage | No eval() in any .ts/.tsx file | PASS |

## CSP — PASS

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| CSP header on all responses | `SecurityHeadersMiddleware` in `internal/server/middleware.go` | PASS |
| default-src 'self' | In CSP header value | PASS |
| script-src 'self' | No inline scripts in frontend; CSP blocks inline | PASS |
| connect-src 'self' ws: wss: | Allows WebSocket connections | PASS |
| img-src 'self' data: | Allows data URIs for images | PASS |
| Meta CSP fallback | `frontend/index.html` contains CSP meta tag | PASS |
| X-Content-Type-Options | nosniff | PASS |
| X-Frame-Options | DENY | PASS |
| Referrer-Policy | strict-origin-when-cross-origin | PASS |

## CORS — PASS

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| Origin allowlist | `CorsMiddleware` allows localhost:5173, 127.0.0.1:5173 | PASS |
| Preflight handled | OPTIONS → 204 with headers | PASS |
| Allowed methods | GET, POST, PUT, DELETE, OPTIONS | PASS |
| Allowed headers | Authorization, Content-Type | PASS |
| Credentials | Access-Control-Allow-Credentials: true | PASS |

## WebSocket Security — PASS

| Requirement | Implementation | Status |
|-------------|---------------|--------|
| 30s ping interval | `wsPingInterval = 30 * time.Second` | PASS |
| 10s pong timeout | `wsPongTimeout = 10 * time.Second` | PASS |
| 3 failed pongs → disconnect | `wsMaxPongFail = 3` | PASS |
| Origin validation on upgrade | `upgrader.CheckOrigin` validates origin | PASS |
| Auth context carries to WS | Handler reads UserID from request context | PASS |
| Per-pipeline engine isolation | Separate QueryEngine per pipelineID | PASS |

## Open Items (Phase 3+)

- [ ] SSO/OIDC integration (currently dev-mode any-username login)
- [ ] mTLS for internal service communication
- [ ] Session table in database (currently stateless JWT)
- [ ] Rate limiter wired into route chain (implementation exists, not wired)
- [ ] Token rotation on refresh (stateless JWT does not support revocation yet)
