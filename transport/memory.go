package transport

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MemoryTransport is an in-memory, deterministic transport for testing.
// All operations are synchronous and fully ordered.
// Useful for testing distributed algorithms without network complexity.
type MemoryTransport struct {
	mu        sync.RWMutex
	nodes     map[string]*memoryNode          // Address -> Node
	listeners map[string]*memoryListener      // Address -> Listener
	routes    map[string]map[string]*memQueue // From -> (To -> Queue)
	config    TransportConfig
	closed    bool
}

// memoryNode represents a node in the memory transport.
type memoryNode struct {
	addr      string
	transport *MemoryTransport
}

// memQueue is a deterministic, ordered queue for in-memory message passing.
type memQueue struct {
	mu       sync.Mutex
	messages []*Message
	notifier chan struct{}
	closed   bool
}

// memoryListener implements Listener for in-memory transport.
type memoryListener struct {
	addr         string
	transport    *MemoryTransport
	connections  chan Connection
	closed       bool
	mu           sync.Mutex
	deliveryTime time.Duration // Configurable delivery latency for testing
}

// memoryConnection implements Connection for in-memory transport.
type memoryConnection struct {
	localAddr  string
	remoteAddr string
	transport  *MemoryTransport
	inbound    *memQueue // Messages coming TO this connection
	outbound   *memQueue // Messages going FROM this connection
	closed     bool
	mu         sync.Mutex
}

// NewMemoryTransport creates a new in-memory deterministic transport.
func NewMemoryTransport(config TransportConfig) *MemoryTransport {
	if config.NodeID == "" {
		config.NodeID = "memory-node"
	}
	return &MemoryTransport{
		nodes:     make(map[string]*memoryNode),
		listeners: make(map[string]*memoryListener),
		routes:    make(map[string]map[string]*memQueue),
		config:    config,
	}
}

// Listen creates a listener on the given local address.
func (t *MemoryTransport) Listen(ctx context.Context, localAddr string) (Listener, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, ErrListenerClosed
	}

	if _, exists := t.listeners[localAddr]; exists {
		return nil, fmt.Errorf("listener already exists on %s", localAddr)
	}

	listener := &memoryListener{
		addr:        localAddr,
		transport:   t,
		connections: make(chan Connection, 128), // Reasonable default buffer
	}

	t.listeners[localAddr] = listener
	t.routes[localAddr] = make(map[string]*memQueue)

	return listener, nil
}

// Dial creates an outbound connection to the given remote address.
func (t *MemoryTransport) Dial(ctx context.Context, remoteAddr string) (Connection, error) {
	t.mu.RLock()
	listener, exists := t.listeners[remoteAddr]
	t.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no listener at %s", remoteAddr)
	}

	if t.closed {
		return nil, ErrConnectionClosed
	}

	// Create the client-side connection
	clientOutbound := &memQueue{
		notifier: make(chan struct{}, 1),
	}
	clientInbound := &memQueue{
		notifier: make(chan struct{}, 1),
	}

	clientConn := &memoryConnection{
		localAddr:  t.config.NodeID,
		remoteAddr: remoteAddr,
		transport:  t,
		inbound:    clientInbound,
		outbound:   clientOutbound,
	}

	// Create the server-side connection
	serverOutbound := clientInbound // Server sends on client's inbound
	serverInbound := clientOutbound // Server receives from client's outbound

	serverConn := &memoryConnection{
		localAddr:  remoteAddr,
		remoteAddr: t.config.NodeID,
		transport:  t,
		inbound:    serverInbound,
		outbound:   serverOutbound,
	}

	// Deliver server-side connection to listener
	select {
	case listener.connections <- serverConn:
		return clientConn, nil
	case <-ctx.Done():
		return nil, ErrContextCancelled
	default:
		return nil, fmt.Errorf("listener connection queue full")
	}
}

// Close closes the transport.
func (t *MemoryTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	// Close all listeners
	for _, listener := range t.listeners {
		listener.mu.Lock()
		if !listener.closed {
			listener.closed = true
			close(listener.connections)
		}
		listener.mu.Unlock()
	}

	// Could cleanup routes here if needed
	return nil
}

// --- Listener Implementation ---

// Accept waits for the next inbound connection.
func (l *memoryListener) Accept(ctx context.Context) (Connection, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, ErrListenerClosed
	}
	l.mu.Unlock()

	select {
	case conn := <-l.connections:
		return conn, nil
	case <-ctx.Done():
		return nil, ErrContextCancelled
	}
}

// Close closes the listener.
func (l *memoryListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closed = true
	close(l.connections)
	return nil
}

// Addr returns the listener's address.
func (l *memoryListener) Addr() string {
	return l.addr
}

// --- Connection Implementation ---

// Send transmits a message.
func (c *memoryConnection) Send(ctx context.Context, msg *Message) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrConnectionClosed
	}
	c.mu.Unlock()

	if msg == nil {
		return fmt.Errorf("cannot send nil message")
	}

	// Optionally add delivery latency for more realistic testing
	select {
	case <-ctx.Done():
		return ErrContextCancelled
	case <-time.After(0): // No latency by default
	}

	c.outbound.mu.Lock()
	if c.outbound.closed {
		c.outbound.mu.Unlock()
		return ErrConnectionClosed
	}

	c.outbound.messages = append(c.outbound.messages, msg)
	c.outbound.mu.Unlock()

	// Notify any waiters
	select {
	case c.outbound.notifier <- struct{}{}:
	default:
	}

	return nil
}

// Recv receives a message.
func (c *memoryConnection) Recv(ctx context.Context) (*Message, error) {
	for {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return nil, ErrConnectionClosed
		}
		c.mu.Unlock()

		c.inbound.mu.Lock()
		if len(c.inbound.messages) > 0 {
			msg := c.inbound.messages[0]
			c.inbound.messages = c.inbound.messages[1:]
			c.inbound.mu.Unlock()
			return msg, nil
		}
		closed := c.inbound.closed
		c.inbound.mu.Unlock()

		// If queue is empty and closed, connection is done
		if closed {
			return nil, ErrConnectionClosed
		}

		// Wait for notification or context cancellation
		select {
		case <-ctx.Done():
			return nil, ErrContextCancelled
		case _, ok := <-c.inbound.notifier:
			if !ok {
				// Channel closed, connection is closing
				return nil, ErrConnectionClosed
			}
			// Loop back to check messages
		}
	}
}

// Close closes the connection.
func (c *memoryConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil // Already closed, idempotent
	}
	c.closed = true

	// Close inbound queue and wake any blocked receivers
	c.inbound.mu.Lock()
	if !c.inbound.closed {
		c.inbound.closed = true
		close(c.inbound.notifier)
	}
	c.inbound.mu.Unlock()

	// Close outbound queue (no senders should be blocked on it)
	c.outbound.mu.Lock()
	if !c.outbound.closed {
		c.outbound.closed = true
		close(c.outbound.notifier)
	}
	c.outbound.mu.Unlock()

	return nil
}

// LocalAddr returns the local address.
func (c *memoryConnection) LocalAddr() string {
	return c.localAddr
}

// RemoteAddr returns the remote address.
func (c *memoryConnection) RemoteAddr() string {
	return c.remoteAddr
}

// GetAllMessages returns all messages currently in the outbound queue.
// Useful for testing. Does not remove messages.
func (c *memoryConnection) GetAllMessages() []*Message {
	c.outbound.mu.Lock()
	defer c.outbound.mu.Unlock()

	msgs := make([]*Message, len(c.outbound.messages))
	copy(msgs, c.outbound.messages)
	return msgs
}

// ClearMessages removes all messages from the outbound queue.
// Useful for testing.
func (c *memoryConnection) ClearMessages() {
	c.outbound.mu.Lock()
	c.outbound.messages = c.outbound.messages[:0]
	c.outbound.mu.Unlock()
}
