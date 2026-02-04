package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/getmockd/mockd/pkg/logging"
)

// StdioServer runs the MCP protocol over stdin/stdout (newline-delimited JSON-RPC).
// This is the primary transport for MCP clients like Claude Desktop, Cursor, etc.
//
// Usage in Claude Desktop config:
//
//	{
//	  "mcpServers": {
//	    "mockd": {
//	      "command": "mockd",
//	      "args": ["mcp"]
//	    }
//	  }
//	}
type StdioServer struct {
	server  *Server
	session *MCPSession
	reader  io.Reader
	writer  io.Writer
	log     *slog.Logger
	mu      sync.Mutex
}

// NewStdioServer creates a new stdio MCP server.
// The server parameter provides the dispatch logic, tools, and resources.
func NewStdioServer(server *Server) *StdioServer {
	return &StdioServer{
		server: server,
		reader: os.Stdin,
		writer: os.Stdout,
		log:    logging.Nop(),
	}
}

// SetLogger sets the logger. Logs go to stderr to avoid interfering with the
// stdio protocol on stdout.
func (s *StdioServer) SetLogger(log *slog.Logger) {
	if log != nil {
		s.log = log
	}
}

// SetIO overrides the default stdin/stdout for testing.
func (s *StdioServer) SetIO(reader io.Reader, writer io.Writer) {
	s.reader = reader
	s.writer = writer
}

// Run starts the stdio event loop. Blocks until EOF on stdin or an error.
func (s *StdioServer) Run() error {
	s.log.Info("MCP stdio server starting",
		"version", ServerVersion,
		"protocol", ProtocolVersion,
	)

	scanner := bufio.NewScanner(s.reader)
	// MCP stdio uses newline-delimited JSON; allow up to 10MB per message.
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		s.log.Debug("received", "message", string(line))

		resp := s.handleMessage(line)
		if resp != nil {
			s.writeResponse(resp)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stdin read error: %w", err)
	}

	s.log.Info("MCP stdio server stopped (EOF)")
	return nil
}

// handleMessage processes a single JSON-RPC message and returns the response.
// Returns nil for notifications (which require no response).
func (s *StdioServer) handleMessage(data []byte) *JSONRPCResponse {
	req, parseErr := ParseRequestBytes(data)
	if parseErr != nil {
		return ErrorResponse(nil, parseErr)
	}

	// Initialize creates the single stdio session.
	if req.Method == "initialize" {
		s.session = NewSession()
		s.server.initSession(s.session)
		result, rpcErr := s.server.dispatch(s.session, req)
		if req.IsNotification() {
			return nil
		}
		if rpcErr != nil {
			return ErrorResponse(req.ID, rpcErr)
		}
		return SuccessResponse(req.ID, result)
	}

	// Initialized is a notification — no response needed.
	if req.Method == "initialized" {
		if s.session != nil {
			_, _ = s.server.dispatch(s.session, req)
		}
		return nil
	}

	// All other methods require an initialized session.
	if s.session == nil {
		if req.IsNotification() {
			return nil
		}
		return ErrorResponse(req.ID, NotInitializedError())
	}

	s.session.Touch()

	result, rpcErr := s.server.dispatch(s.session, req)

	if req.IsNotification() {
		return nil
	}
	if rpcErr != nil {
		return ErrorResponse(req.ID, rpcErr)
	}
	return SuccessResponse(req.ID, result)
}

// writeResponse writes a JSON-RPC response as a single line to stdout.
func (s *StdioServer) writeResponse(resp *JSONRPCResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		s.log.Error("failed to marshal response", "error", err)
		return
	}

	s.log.Debug("sending", "message", string(data))

	// Newline-delimited JSON.
	data = append(data, '\n')
	if _, err := s.writer.Write(data); err != nil {
		s.log.Error("failed to write response", "error", err)
	}
}
