package protocol

// StandaloneServer is a handler that runs as an independent server.
// Implement this interface for protocols that listen on their own port
// like gRPC, MQTT, etc.
//
// Example implementation:
//
//	type MyServer struct {
//	    listener net.Listener
//	    running  bool
//	}
//
//	func (s *MyServer) Port() int {
//	    if s.listener == nil {
//	        return 0
//	    }
//	    addr := s.listener.Addr().(*net.TCPAddr)
//	    return addr.Port
//	}
//
//	func (s *MyServer) Address() string {
//	    if s.listener == nil {
//	        return ""
//	    }
//	    return s.listener.Addr().String()
//	}
//
//	func (s *MyServer) IsRunning() bool {
//	    return s.running
//	}
type StandaloneServer interface {
	Handler

	// Port returns the port the server is listening on.
	// Returns 0 if the server is not running or port is not applicable.
	Port() int

	// Address returns the full address the server is listening on.
	// Examples: "0.0.0.0:50051", "[::]:1883"
	// Returns empty string if the server is not running.
	Address() string

	// IsRunning returns true if the server is actively listening.
	IsRunning() bool
}
