package transport

import (
	"context"
	"errors"
	"time"
)

// Message represents a transport-level message with vector clock tracking.
// The body is opaque to the transport layer - serialization is caller's responsibility.
type Message struct {
	// From is the source node ID
	From string

	// To is the destination node ID
	To string

	// Seq is an optional sequence number for ordering
	Seq uint64

	// Body is the raw message payload (opaque to transport)
	Body []byte

	// Timestamp is when the message was sent
	Timestamp time.Time

	// VectorClockData is optional pre-marshalled vector clock data
	// The transport layer treats this as opaque bytes
	VectorClockData []byte

	// Metadata is transport-specific metadata (version, compression, etc)
	Metadata map[string]string
}

// Connection represents a bidirectional communication channel between two nodes.
// Implementations must be safe for concurrent Send and Recv calls.
type Connection interface {
	// Send transmits a message. Returns error if connection is closed or send fails.
	// Send should be non-blocking on success; caller should not assume ordering
	// between concurrent Sends without additional synchronization.
	Send(ctx context.Context, msg *Message) error

	// Recv receives a message. Blocks until a message arrives or context is cancelled.
	// Returns error if connection is closed or receive fails.
	// Recv calls may block indefinitely; caller responsible for context timeout.
	Recv(ctx context.Context) (*Message, error)

	// Close closes the connection. Subsequent Send/Recv calls may return errors.
	// Close is idempotent; calling multiple times should not error.
	Close() error

	// LocalAddr returns the local address of this connection.
	LocalAddr() string

	// RemoteAddr returns the remote address of this connection.
	RemoteAddr() string
}

// Dialer establishes outbound connections to remote nodes.
type Dialer interface {
	// Dial establishes an outbound connection to the given address.
	// Context can be used to apply timeouts. Should return error if dial fails.
	Dial(ctx context.Context, addr string) (Connection, error)
}

// Listener accepts inbound connections from remote nodes.
type Listener interface {
	// Accept waits for and returns the next inbound connection.
	// Context can be used to apply cancel. Should return error if accept fails.
	Accept(ctx context.Context) (Connection, error)

	// Close closes the listener, preventing new accepts.
	Close() error

	// Addr returns the listener's address.
	Addr() string
}

// Transport is a bidirectional network transport implementation.
type Transport interface {
	// Listen creates a listener on the given local address.
	Listen(ctx context.Context, localAddr string) (Listener, error)

	// Dial creates an outbound connection to the given remote address.
	Dial(ctx context.Context, remoteAddr string) (Connection, error)

	// Close closes the transport and all active connections.
	// Existing connections should be closed gracefully.
	Close() error
}

// Common errors
var (
	ErrConnectionClosed = errors.New("connection closed")
	ErrListenerClosed   = errors.New("listener closed")
	ErrDialFailed       = errors.New("dial failed")
	ErrContextCancelled = errors.New("context cancelled")
	ErrMessageTooLarge  = errors.New("message too large")
	ErrInvalidAddress   = errors.New("invalid address")
	ErrNotImplemented   = errors.New("not implemented")
)

// TransportConfig holds common configuration for transport implementations.
type TransportConfig struct {
	// MaxMessageSize is the maximum allowed message size in bytes.
	// Set to 0 for unlimited. Transport may enforce additional limits.
	MaxMessageSize int

	// DialTimeout is the timeout for outbound dial operations.
	// Set to 0 for no timeout.
	DialTimeout time.Duration

	// ConnectRetries is the number of times to retry failed connections.
	// Set to 0 for no retries.
	ConnectRetries int

	// ReadTimeout is the timeout for read operations on connections.
	// Set to 0 for no timeout.
	ReadTimeout time.Duration

	// WriteTimeout is the timeout for write operations on connections.
	// Set to 0 for no timeout.
	WriteTimeout time.Duration

	// NodeID is the identifier for this node in the distributed system.
	NodeID string
}

// DefaultConfig returns a reasonable default configuration.
func DefaultConfig() TransportConfig {
	return TransportConfig{
		MaxMessageSize: 16 * 1024 * 1024, // 16MB
		DialTimeout:    10 * time.Second,
		ConnectRetries: 3,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
	}
}
