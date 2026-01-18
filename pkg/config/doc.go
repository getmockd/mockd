// Package config provides configuration types and utilities for the mock server.
//
// This package defines all the configuration structures used by the mock server:
//   - MockConfiguration: Defines a mock endpoint with matcher and response
//   - RequestMatcher: Specifies criteria for matching incoming requests
//   - ResponseDefinition: Defines the response to return for matched requests
//   - ServerConfiguration: Server settings like ports, timeouts, and TLS options
//   - MockCollection: A collection of mocks for file-based configuration
//
// Request Matching:
//
// Requests are matched based on multiple criteria, each contributing to a score:
//   - Method: HTTP method (GET, POST, etc.)
//   - Path: Exact or wildcard path matching
//   - Headers: Required header key-value pairs
//   - QueryParams: Required query parameter key-value pairs
//   - BodyContains/BodyEquals: Body content matching
//
// File-based Configuration:
//
// Mocks can be loaded from JSON files:
//
//	collection, err := config.LoadFromFile("mocks.json")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// The JSON format follows the MockCollection structure:
//
//	{
//	  "version": "1.0",
//	  "name": "My API Mocks",
//	  "mocks": [
//	    {
//	      "id": "get-users",
//	      "matcher": {"method": "GET", "path": "/api/users"},
//	      "response": {"statusCode": 200, "body": "[]"}
//	    }
//	  ]
//	}
package config
