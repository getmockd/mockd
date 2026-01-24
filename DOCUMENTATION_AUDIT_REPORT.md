# Documentation Audit Report for Public Release

**Audit Date:** January 23, 2026  
**Auditor:** Documentation Review  
**Scope:** All documentation in `/mockd/docs/`

---

## CRITICAL FIXES (Blocking Release)

### 1. HTTP Request Matching - `request` vs `matcher` field inconsistency
**File:** `docs/guides/request-matching.md`  
**Issue:** Documentation uses both `request` and `matcher` field names interchangeably. The actual implementation uses `matcher` in the Mock struct.
**Evidence:** 
- Lines 11-16, 24-31, 36-43 use `"request": {...}` 
- Lines 96-101, 110-114, 291-296, 309-323 use `"matcher": {...}`
**Fix:** Standardize ALL examples to use `matcher` which is the actual field name in the implementation (`pkg/mock/types.go`).

### 2. HTTP Query/Header Regex Matching - NOT IMPLEMENTED
**File:** `docs/guides/request-matching.md`  
**Issue:** Documentation shows regex patterns for query params (lines 148-158) and headers (lines 196-205), but implementation uses EXACT matching only.
**Evidence:** `internal/matching/query.go:8-11` shows `MatchQueryParam` does exact equality check: `return actualValue == expectedValue`
**Fix:** Remove regex examples from query matching section OR implement regex support. Keep header wildcards (`*`) documentation which IS implemented in `MatchHeaderPattern`.

### 3. HTTP Header Negation Syntax - NOT IMPLEMENTED  
**File:** `docs/guides/request-matching.md`  
**Issue:** Documentation shows `!` prefix negation syntax (lines 420-435, 452-468) but this is NOT implemented in the matching code.
**Evidence:** `internal/matching/matcher.go` and `internal/matching/header.go` have no negation logic.
**Fix:** Remove negation examples OR implement negation support.

### 4. gRPC Inline Proto Definitions - NOT SUPPORTED
**File:** `docs/guides/grpc-mocking.md`  
**Issue:** Quick start example (lines 30-45) shows inline proto definition in YAML with `protoFile: |`. The implementation requires file paths.
**Evidence:** `pkg/grpc/types.go:24` shows `ProtoFile string` expects a file path, not inline content.
**Fix:** Update all inline proto examples to use external `.proto` files with proper `protoFile:` path references.

### 5. gRPC Template Variables - NOT IMPLEMENTED
**File:** `docs/guides/grpc-mocking.md`  
**Issue:** Documentation shows `{{uuid}}`, `{{now}}`, `{{timestamp}}` templates in gRPC responses (lines 282-292, 404-424, 762-776).
**Evidence:** Grep for these patterns in `pkg/graphql/` and `pkg/grpc/` shows no template processing.
**Fix:** Either implement template processing for gRPC responses OR remove all template variable examples from gRPC docs.

### 6. GraphQL Template Variables - NOT IMPLEMENTED
**File:** `docs/guides/graphql-mocking.md`  
**Issue:** Documentation shows `{{uuid}}`, `{{now}}`, `{{timestamp}}`, `{{args.*}}` templates (lines 313-336, 506-513, 779-792).
**Evidence:** No template processing found in `pkg/graphql/` package.
**Fix:** Either implement template processing for GraphQL resolvers OR remove template examples.

### 7. SOAP Template Variables - NOT IMPLEMENTED  
**File:** `docs/guides/soap-mocking.md`  
**Issue:** Documentation shows `{{uuid}}`, `{{now}}`, `{{timestamp}}` in SOAP responses (lines 310-324, 693-698, 861-866).
**Evidence:** No template processing in SOAP handler code.
**Fix:** Either implement template processing OR remove examples.

### 8. MQTT ACL Access Values Mismatch
**File:** `docs/guides/mqtt-mocking.md`  
**Issue:** Documentation shows access values `publish`/`subscribe`/`all`/`deny` (lines 368-376, 392-401) but implementation uses `read`/`write`/`readwrite`.
**Evidence:** `pkg/mqtt/types.go:39` shows `Access string // "read", "write", "readwrite"` and `pkg/mqtt/hooks.go:123-134` shows only `readwrite`, `read`, `write` cases.
**Fix:** Update documentation to use correct values: `read`, `write`, `readwrite` instead of `publish`, `subscribe`, `all`.

### 9. Stateful Mocking - Wrong Configuration Schema
**File:** `docs/guides/stateful-mocking.md`  
**Issue:** Documentation shows JSON with `"stateful": { "resources": {...} }` schema (lines 20-32, 71-82, 92-112) but actual implementation uses different structure.
**Evidence:** `pkg/config/types.go:197-209` shows `StatefulResourceConfig` has fields: `Name`, `BasePath`, `IDField`, `ParentField`, `SeedData`. The `collection`/`item` paths shown in docs don't exist.
**Fix:** Complete rewrite needed. Replace all examples with actual schema:
```yaml
statefulResources:
  - name: users
    basePath: /api/users
    idField: id
    seedData: [...]
```

### 10. Stateful Mocking - Persistence NOT IMPLEMENTED
**File:** `docs/guides/stateful-mocking.md`  
**Issue:** Documentation describes file persistence (lines 346-363) with `persistence: { enabled: true, file: ... }` but this is NOT implemented.
**Evidence:** No persistence logic in `pkg/stateful/` - state is in-memory only.
**Fix:** Remove persistence section OR implement it.

---

## HIGH PRIORITY FIXES

### 11. SSE Timing Field Names
**File:** `docs/guides/sse-streaming.md`  
**Issue:** Documentation shows `timing: { fixedDelay: ... }` (lines 57-61, 119-127) but actual field names may differ.
**Status:** Verify against SSE implementation - field names need confirmation.
**Fix:** Verify and correct field names in timing configuration.

### 12. OAuth PKCE Documentation Missing
**File:** Not documented  
**Issue:** `pkg/oauth/doc.go` mentions "Authorization Code (with PKCE support)" but there's no documentation for PKCE configuration.
**Fix:** Add PKCE documentation if feature exists, or clarify limitations.

### 13. MQTT Template Syntax Inconsistency
**File:** `docs/guides/mqtt-mocking.md`  
**Issue:** Documentation shows `{{randomInt min max}}` Go template syntax (lines 203-224) but actual implementation uses different patterns.
**Evidence:** `pkg/mqtt/template.go` shows pattern `random.int(min, max)` not `randomInt min max`.
**Fix:** Update template syntax examples to match actual implementation:
- `{{ random.int(min, max) }}` instead of `{{randomInt min max}}`
- `{{ timestamp.iso }}` instead of `{{now}}`

### 14. CLI Command Documentation - Verify All Commands
**File:** `docs/reference/cli.md`  
**Issue:** Extensive CLI documentation - needs verification that all documented commands and flags exist.
**Priority:** Spot-check key commands like `mockd serve`, `mockd add`, `mockd import`.

### 15. Admin API Endpoints - Missing Some Endpoints
**File:** `docs/reference/admin-api.md`  
**Issue:** Some endpoints documented may not exist or have different paths.
**Priority:** Verify endpoints against actual router registration in `pkg/admin/`.

### 16. Quickstart - Uses Correct Schema
**File:** `docs/getting-started/quickstart.md`  
**Status:** VERIFIED CORRECT - Uses `matcher` field name properly.

---

## MEDIUM PRIORITY FIXES

### 17. gRPC CLI Commands - May Not Exist
**File:** `docs/guides/grpc-mocking.md`  
**Issue:** Documents `mockd grpc list` and `mockd grpc call` commands (lines 988-1029) - need verification.

### 18. SOAP CLI Commands - May Not Exist
**File:** `docs/guides/soap-mocking.md`  
**Issue:** Documents `mockd soap validate` and `mockd soap call` commands (lines 908-968) - need verification.

### 19. GraphQL CLI Commands - Verify Existence
**File:** `docs/guides/graphql-mocking.md`  
**Issue:** Documents `mockd graphql validate` and `mockd graphql query` commands - need verification.

### 20. Multiple Protocol Docs Show Inline Schema Definitions
**Files:** Multiple  
**Issue:** Several docs show inline WSDL/proto/schema definitions that may not work as shown.

### 21. Request Matching - bodyMatch JSONPath Example
**File:** `docs/guides/request-matching.md`  
**Issue:** Lines 259-272 show `bodyMatch` field but implementation uses `bodyJsonPath`.
**Fix:** Rename `bodyMatch` to `bodyJsonPath` in examples.

### 22. Response Templating Cross-Reference
**File:** `docs/guides/response-templating.md`  
**Issue:** Need to verify this file exists and accurately describes HTTP response templating.

---

## Summary

| Category | Count | Details |
|----------|-------|---------|
| **CRITICAL** | 10 | Blocking release - incorrect/missing features |
| **HIGH** | 6 | Significant inaccuracies |
| **MEDIUM** | 6 | Minor issues or unverified claims |
| **TOTAL** | 22 | Issues requiring attention |

---

## Recommended Actions

### Immediate (Before Release)

1. **Fix all CRITICAL issues** - These will cause user confusion and support burden
2. **Standardize field names** - `matcher` not `request` throughout all docs
3. **Remove unimplemented features** - Templates in gRPC/GraphQL/SOAP, negation syntax, regex query matching
4. **Rewrite Stateful Mocking guide** - Current schema is completely wrong

### Short-Term (Release +1 week)

1. Verify all CLI commands exist
2. Verify all Admin API endpoints
3. Create integration tests for documented examples

### Medium-Term (Release +1 month)

1. Implement missing features if desired (templates in gRPC/GraphQL/SOAP, persistence)
2. Add automated doc testing to CI pipeline

---

## Files Requiring Changes

| File | Priority | Scope |
|------|----------|-------|
| `docs/guides/request-matching.md` | CRITICAL | Major rewrite |
| `docs/guides/stateful-mocking.md` | CRITICAL | Complete rewrite |
| `docs/guides/grpc-mocking.md` | CRITICAL | Remove inline proto, templates |
| `docs/guides/graphql-mocking.md` | CRITICAL | Remove templates |
| `docs/guides/soap-mocking.md` | CRITICAL | Remove templates |
| `docs/guides/mqtt-mocking.md` | CRITICAL | Fix ACL values, template syntax |
| `docs/guides/sse-streaming.md` | HIGH | Verify field names |
| `docs/reference/cli.md` | HIGH | Verify commands |
| `docs/reference/admin-api.md` | HIGH | Verify endpoints |
| `docs/getting-started/quickstart.md` | OK | No changes needed |
