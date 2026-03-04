package transport

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// TCPTransport implements Transport over TCP with binary message framing.
// Messages are framed with a 4-byte big-endian length prefix followed by the message data.
type TCPTransport struct {
	mu          sync.RWMutex
	listener    net.Listener
	connections map[string]*TCPConnection
	config      TransportConfig
	activeConns atomic.Int32
	closed      bool
	closeOnce   sync.Once
}

// TCPConnection implements Connection over a TCP socket.
type TCPConnection struct {
	mu        sync.RWMutex
	conn      net.Conn
	reader    *bufio.Reader
	writer    *bufio.Writer
	closed    bool
	config    TransportConfig
	sendMu    sync.Mutex
	recvMu    sync.Mutex
	sendCount atomic.Int64
	recvCount atomic.Int64
}

// TCPListener implements Listener for TCP connections.
type TCPListener struct {
	mu      sync.Mutex
	tcpList net.Listener
	config  TransportConfig
	closed  bool
}

// NewTCPTransport creates a new TCP-based transport.
func NewTCPTransport(config TransportConfig) *TCPTransport {
	if config.MaxMessageSize == 0 {
		config.MaxMessageSize = 16 * 1024 * 1024 // 16MB default
	}
	if config.NodeID == "" {
		config.NodeID = "tcp-node"
	}
	return &TCPTransport{
		connections: make(map[string]*TCPConnection),
		config:      config,
	}
}

// Listen creates a TCP listener on the given address.
func (t *TCPTransport) Listen(ctx context.Context, localAddr string) (Listener, error) {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return nil, ErrListenerClosed
	}
	t.mu.Unlock()

	tcpAddr, err := net.ResolveTCPAddr("tcp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	tcpList, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, fmt.Errorf("listen failed: %w", err)
	}

	return &TCPListener{
		tcpList: tcpList,
		config:  t.config,
	}, nil
}

// Dial creates an outbound TCP connection to the given address.
func (t *TCPTransport) Dial(ctx context.Context, remoteAddr string) (Connection, error) {
	t.mu.RLock()
	if t.closed {
		t.mu.RUnlock()
		return nil, ErrConnectionClosed
	}
	t.mu.RUnlock()

	dialer := &net.Dialer{}
	if t.config.DialTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, t.config.DialTimeout)
		defer cancel()
	}

	conn, err := dialer.DialContext(ctx, "tcp", remoteAddr)
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	tcpConn := &TCPConnection{
		conn:   conn,
		reader: bufio.NewReaderSize(conn, 64*1024),
		writer: bufio.NewWriterSize(conn, 64*1024),
		config: t.config,
	}

	t.mu.Lock()
	t.connections[remoteAddr] = tcpConn
	t.mu.Unlock()

	t.activeConns.Add(1)
	return tcpConn, nil
}

// Close closes the transport and all connections.
func (t *TCPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true

	for _, conn := range t.connections {
		_ = conn.Close()
	}

	t.connections = make(map[string]*TCPConnection)
	return nil
}

// --- TCPListener Implementation ---

// Accept waits for a TCP connection.
func (l *TCPListener) Accept(ctx context.Context) (Connection, error) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, ErrListenerClosed
	}
	l.mu.Unlock()

	// Use channel to make it cancellable by context
	type result struct {
		conn net.Conn
		err  error
	}
	connChan := make(chan result, 1)

	go func() {
		conn, err := l.tcpList.Accept()
		connChan <- result{conn, err}
	}()

	select {
	case res := <-connChan:
		if res.err != nil {
			return nil, fmt.Errorf("accept failed: %w", res.err)
		}

		tcpConn := &TCPConnection{
			conn:   res.conn,
			reader: bufio.NewReaderSize(res.conn, 64*1024),
			writer: bufio.NewWriterSize(res.conn, 64*1024),
			config: l.config,
		}

		return tcpConn, nil

	case <-ctx.Done():
		return nil, ErrContextCancelled
	}
}

// Close closes the listener.
func (l *TCPListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.closed = true
	return l.tcpList.Close()
}

// Addr returns the listener's address.
func (l *TCPListener) Addr() string {
	return l.tcpList.Addr().String()
}

// --- TCPConnection Implementation ---

// Send transmits a message using binary framing.
// Format: [4-byte big-endian length][message body]
func (c *TCPConnection) Send(ctx context.Context, msg *Message) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrConnectionClosed
	}
	c.mu.RUnlock()

	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	// Serialize message
	data := serializeMessage(msg)

	if c.config.MaxMessageSize > 0 && len(data) > c.config.MaxMessageSize {
		return ErrMessageTooLarge
	}

	// Write length prefix
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(data)))

	if c.config.WriteTimeout > 0 {
		c.conn.SetWriteDeadline(time.Now().Add(c.config.WriteTimeout))
	}

	if _, err := c.writer.Write(lenBuf); err != nil {
		c.close()
		return fmt.Errorf("write length failed: %w", err)
	}

	if _, err := c.writer.Write(data); err != nil {
		c.close()
		return fmt.Errorf("write data failed: %w", err)
	}

	if err := c.writer.Flush(); err != nil {
		c.close()
		return fmt.Errorf("flush failed: %w", err)
	}

	c.sendCount.Add(1)
	return nil
}

// Recv receives a message using binary framing.
func (c *TCPConnection) Recv(ctx context.Context) (*Message, error) {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return nil, ErrConnectionClosed
	}
	c.mu.RUnlock()

	c.recvMu.Lock()
	defer c.recvMu.Unlock()

	if c.config.ReadTimeout > 0 {
		c.conn.SetReadDeadline(time.Now().Add(c.config.ReadTimeout))
	}

	// Read length prefix
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(c.reader, lenBuf); err != nil {
		c.close()
		return nil, fmt.Errorf("read length failed: %w", err)
	}

	msgLen := binary.BigEndian.Uint32(lenBuf)

	if c.config.MaxMessageSize > 0 && int(msgLen) > c.config.MaxMessageSize {
		c.close()
		return nil, ErrMessageTooLarge
	}

	// Read message data
	data := make([]byte, msgLen)
	if _, err := io.ReadFull(c.reader, data); err != nil {
		c.close()
		return nil, fmt.Errorf("read data failed: %w", err)
	}

	msg, err := deserializeMessage(data)
	if err != nil {
		return nil, fmt.Errorf("deserialize failed: %w", err)
	}

	c.recvCount.Add(1)
	return msg, nil
}

// Close closes the connection.
func (c *TCPConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.close()
}

func (c *TCPConnection) close() error {
	if c.closed {
		return nil
	}
	c.closed = true
	return c.conn.Close()
}

// LocalAddr returns the local address.
func (c *TCPConnection) LocalAddr() string {
	return c.conn.LocalAddr().String()
}

// RemoteAddr returns the remote address.
func (c *TCPConnection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

// Message Serialization
// Simple binary format: [fields serialized sequentially]

func serializeMessage(msg *Message) []byte {
	// Format: [len(from)][from][len(to)][to][seq][len(body)][body][len(vc)][vc]
	data := make([]byte, 0, len(msg.From)+len(msg.To)+len(msg.Body)+len(msg.VectorClockData)+64)

	// From
	data = append(data, byte(len(msg.From)))
	data = append(data, []byte(msg.From)...)

	// To
	data = append(data, byte(len(msg.To)))
	data = append(data, []byte(msg.To)...)

	// Seq
	seqBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(seqBuf, msg.Seq)
	data = append(data, seqBuf...)

	// Body length and data
	bodyLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(bodyLenBuf, uint32(len(msg.Body)))
	data = append(data, bodyLenBuf...)
	data = append(data, msg.Body...)

	// VectorClockData length and data
	vcLenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(vcLenBuf, uint32(len(msg.VectorClockData)))
	data = append(data, vcLenBuf...)
	data = append(data, msg.VectorClockData...)

	return data
}

func deserializeMessage(data []byte) (*Message, error) {
	if len(data) < 19 { // Minimum: 1+1+8+4+4 for required fields
		return nil, fmt.Errorf("data too short")
	}

	msg := &Message{}
	offset := 0

	// From
	fromLen := int(data[offset])
	offset++
	if offset+fromLen > len(data) {
		return nil, fmt.Errorf("invalid from length")
	}
	msg.From = string(data[offset : offset+fromLen])
	offset += fromLen

	// To
	toLen := int(data[offset])
	offset++
	if offset+toLen > len(data) {
		return nil, fmt.Errorf("invalid to length")
	}
	msg.To = string(data[offset : offset+toLen])
	offset += toLen

	// Seq
	if offset+8 > len(data) {
		return nil, fmt.Errorf("invalid seq")
	}
	msg.Seq = binary.BigEndian.Uint64(data[offset : offset+8])
	offset += 8

	// Body
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid body length")
	}
	bodyLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	if offset+int(bodyLen) > len(data) {
		return nil, fmt.Errorf("invalid body data")
	}
	msg.Body = data[offset : offset+int(bodyLen)]
	offset += int(bodyLen)

	// VectorClockData
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid vector clock length")
	}
	vcLen := binary.BigEndian.Uint32(data[offset : offset+4])
	offset += 4
	if offset+int(vcLen) > len(data) {
		return nil, fmt.Errorf("invalid vector clock data")
	}
	msg.VectorClockData = data[offset : offset+int(vcLen)]

	return msg, nil
}
