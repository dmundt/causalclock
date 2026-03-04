package transport

import (
	"context"
	"sync"
	"sync/atomic"
)

// MockTransport is a fully controllable mock transport for testing.
// It allows fine-grained control over message delivery and failure scenarios.
type MockTransport struct {
	mu              sync.RWMutex
	connections     map[string]*MockConnection
	listeners       map[string]*MockListener
	closed          bool
	config          TransportConfig
	onSend          func(*Message) error            // Hook called on Send
	onRecv          func(*Message)                  // Hook called on Recv
	onDial          func(addr string) error         // Hook called on Dial
	deliveryDelay   func(*Message) bool             // Return true to drop
	messageDropper  func(*Message) bool             // Return true to drop message
}

// MockConnection is a controllable test connection.
type MockConnection struct {
	mu              sync.Mutex
	localAddr       string
	remoteAddr      string
	closed          bool
	send_count      atomic.Int64
	recv_count      atomic.Int64
	messages        chan *Message
	transport       *MockTransport
	onSend          func(*Message) error // Override for this connection
	onRecv          func(*Message)        // Override for this connection
	willFailSend    bool
	willFailRecv    bool
	sendErrorCount  int
	sendErrorAfter  int
}

// MockListener is a controllable test listener.
type MockListener struct {
	mu              sync.Mutex
	addr            string
	transport       *MockTransport
	connections     chan Connection
	closed          bool
	acceptCount     atomic.Int64
	acceptErrorNext bool
}

// NewMockTransport creates a new controllable mock transport.
func NewMockTransport(config TransportConfig) *MockTransport {
	if config.NodeID == "" {
		config.NodeID = "mock-node"
	}
	return &MockTransport{
		connections: make(map[string]*MockConnection),
		listeners:   make(map[string]*MockListener),
		config:      config,
	}
}

// Listen creates a mock listener.
func (t *MockTransport) Listen(ctx context.Context, localAddr string) (Listener, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil, ErrListenerClosed
	}

	listener := &MockListener{
		addr:        localAddr,
		transport:   t,
		connections: make(chan Connection, 64),
	}

	t.listeners[localAddr] = listener
	return listener, nil
}

// Dial creates a mock connection.
func (t *MockTransport) Dial(ctx context.Context, remoteAddr string) (Connection, error) {
	t.mu.RLock()
	onDial := t.onDial
	listener, exists := t.listeners[remoteAddr]
	t.mu.RUnlock()

	if onDial != nil {
		if err := onDial(remoteAddr); err != nil {
			return nil, err
		}
	}

	if !exists {
		return nil, ErrDialFailed
	}

	conn := &MockConnection{
		localAddr:   t.config.NodeID,
		remoteAddr:  remoteAddr,
		transport:   t,
		messages:    make(chan *Message, 64),
	}

	t.mu.Lock()
	t.connections[remoteAddr] = conn
	t.mu.Unlock()

	// Deliver to listener
	select {
	case listener.connections <- conn:
		return conn, nil
	case <-ctx.Done():
		return nil, ErrContextCancelled
	}
}

// Close closes the mock transport.
func (t *MockTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}

	t.closed = true

	for _, listener := range t.listeners {
		listener.mu.Lock()
		if !listener.closed {
			listener.closed = true
			close(listener.connections)
		}
		listener.mu.Unlock()
	}

	return nil
}

// SetOnSend sets a global send hook.
func (t *MockTransport) SetOnSend(hook func(*Message) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onSend = hook
}

// SetOnRecv sets a global recv hook.
func (t *MockTransport) SetOnRecv(hook func(*Message)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onRecv = hook
}

// SetOnDial sets a global dial hook.
func (t *MockTransport) SetOnDial(hook func(addr string) error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onDial = hook
}

// SetMessageDropper sets a function that determines if a message should be dropped.
func (t *MockTransport) SetMessageDropper(dropper func(*Message) bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.messageDropper = dropper
}

// GetConnection returns a connection by remote address.
func (t *MockTransport) GetConnection(remoteAddr string) *MockConnection {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.connections[remoteAddr]
}

// --- MockConnection Implementation ---

// Send implements Connection.Send
func (c *MockConnection) Send(ctx context.Context, msg *Message) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return ErrConnectionClosed
	}

	if c.onSend != nil {
		c.mu.Unlock()
		return c.onSend(msg)
	}

	if c.willFailSend && (c.sendErrorCount == 0 || c.send_count.Load() >= int64(c.sendErrorAfter)) {
		c.mu.Unlock()
		c.send_count.Add(1)
		return ErrDialFailed
	}

	c.mu.Unlock()

	// Check global handler
	t := c.transport
	t.mu.RLock()
	onSend := t.onSend
	dropper := t.messageDropper
	t.mu.RUnlock()

	if onSend != nil {
		if err := onSend(msg); err != nil {
			c.send_count.Add(1)
			return err
		}
	}

	if dropper != nil && dropper(msg) {
		c.send_count.Add(1)
		return nil // Silently dropped
	}

	c.send_count.Add(1)

	select {
	case c.messages <- msg:
		return nil
	case <-ctx.Done():
		return ErrContextCancelled
	}
}

// Recv implements Connection.Recv
func (c *MockConnection) Recv(ctx context.Context) (*Message, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrConnectionClosed
	}

	if c.willFailRecv {
		c.mu.Unlock()
		c.recv_count.Add(1)
		return nil, ErrDialFailed
	}

	c.mu.Unlock()

	select {
	case msg := <-c.messages:
		c.recv_count.Add(1)

		// Check global recv hook
		t := c.transport
		t.mu.RLock()
		onRecv := t.onRecv
		t.mu.RUnlock()

		if onRecv != nil {
			onRecv(msg)
		}

		// Check connection-specific recv hook
		c.mu.Lock()
		connRecv := c.onRecv
		c.mu.Unlock()

		if connRecv != nil {
			connRecv(msg)
		}

		return msg, nil
	case <-ctx.Done():
		return nil, ErrContextCancelled
	}
}

// Close implements Connection.Close
func (c *MockConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	close(c.messages)
	return nil
}

// LocalAddr implements Connection.LocalAddr
func (c *MockConnection) LocalAddr() string {
	return c.localAddr
}

// RemoteAddr implements Connection.RemoteAddr
func (c *MockConnection) RemoteAddr() string {
	return c.remoteAddr
}

// SetFailSend makes this connection fail on Send.
func (c *MockConnection) SetFailSend(fail bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.willFailSend = fail
}

// SetFailRecv makes this connection fail on Recv.
func (c *MockConnection) SetFailRecv(fail bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.willFailRecv = fail
}

// SetOnSend sets a connection-specific send hook.
func (c *MockConnection) SetOnSend(hook func(*Message) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onSend = hook
}

// SetOnRecv sets a connection-specific recv hook.
func (c *MockConnection) SetOnRecv(hook func(*Message)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onRecv = hook
}

// SendCount returns the number of messages sent.
func (c *MockConnection) SendCount() int64 {
	return c.send_count.Load()
}

// RecvCount returns the number of messages received.
func (c *MockConnection) RecvCount() int64 {
	return c.recv_count.Load()
}

// PeekMessage returns the next message without removing it.
func (c *MockConnection) PeekMessage() *Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case msg := <-c.messages:
		// Put it back
		c.messages <- msg
		return msg
	default:
		return nil
	}
}

// --- MockListener Implementation ---

// Accept implements Listener.Accept
func (l *MockListener) Accept(ctx context.Context) (Connection, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, ErrListenerClosed
	}

	if l.acceptErrorNext {
		l.acceptErrorNext = false
		l.mu.Unlock()
		return nil, ErrDialFailed
	}

	l.mu.Unlock()

	l.acceptCount.Add(1)

	select {
	case conn := <-l.connections:
		return conn, nil
	case <-ctx.Done():
		return nil, ErrContextCancelled
	}
}

// Close implements Listener.Close
func (l *MockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.closed {
		return nil
	}

	l.closed = true
	close(l.connections)
	return nil
}

// Addr implements Listener.Addr
func (l *MockListener) Addr() string {
	return l.addr
}

// SetAcceptErrorNext makes the next Accept call fail.
func (l *MockListener) SetAcceptErrorNext(fail bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.acceptErrorNext = fail
}

// AcceptCount returns the number of connections accepted.
func (l *MockListener) AcceptCount() int64 {
	return l.acceptCount.Load()
}
