// Package api provides the Engine Control API for internal management.
//
// WARNING: INTERNAL USE ONLY — DO NOT EXPOSE OR USE DIRECTLY
//
// This API is used exclusively by the Admin API (via [engineclient.Client])
// to manage the engine. It is bound to 127.0.0.1, has NO authentication,
// NO authorization, NO rate limiting, and NO CORS protection. It must never
// be exposed on a public network or called directly by end users.
//
// The correct way to manage mocks, request logs, chaos injection, stateful
// resources, and all other engine features is through the Admin REST API
// on port 4290 (default), which provides API key authentication, rate
// limiting, CORS, and full request validation.
//
// Architecture:
//
//	User/CLI/UI → Admin API (:4290) → engineclient → Engine Control API (127.0.0.1:4281+)
//
// If you are reading this and considering calling this API directly — don't.
// Use the Admin API instead.
package api
