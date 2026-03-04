# Transport Abstraction for Vector Clock-Based Distributed Systems

## Overview

The transport package provides a minimal, transport-agnostic abstraction layer for building distributed systems with vector clocks. It cleanly separates concerns between:

- **Message Transport**: How messages physically move between nodes
- **Message Framing**: How messages are delimited (handled internally)
- **Serialization**: How application data is encoded (caller's responsibility)
- **Vector Clock Logic**: Pure causality tracking (separate package)

## Design Philosophy

### 1. Minimal Interface

The core interfaces are deliberately minimal to support diverse implementations:

```go
// Message - transport-level message (opaque body)
type Message struct {
    From string
    To string
    Seq uint64
    Body []byte                  // Opaque to transport
    Timestamp time.Time
    VectorClockData []byte       // Opaque bytes (caller serializes)
    Metadata map[string]string
}

// Connection - bidirectional communication channel
type Connection interface {
    Send(ctx context.Context, msg *Message) error
    Recv(ctx context.Context) (*Message, error)
    Close() error
    LocalAddr() string
    RemoteAddr() string
}

// Transport - establish connections
type Transport interface {
    Listen(ctx context.Context, addr string) (Listener, error)
    Dial(ctx context.Context, addr string) (Connection, error)
    Close() error
}
```

### 2. No Serialization Assumptions

The transport layer treats all message bodies as opaque bytes. This means:

- **No dependency on protocol buffers, JSON, or any specific serialization**
- Caller controls how to encode vector clocks, application data, etc.
- Easy to add compression, encryption, or custom serialization
- Composable with any encoding scheme

### 3. Context-Driven Cancellation

All blocking operations use `context.Context` for cancellation:

```go
// Can apply timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

msg, err := conn.Recv(ctx)
```

Benefits:
- No hidden global state
- Caller controls timeout duration
- Natural cancellation propagation
- Easy graceful shutdown

### 4. Transport-Agnostic Design

Multiple implementations provided:

| Implementation | Use Case | Advantages |
|---|---|---|
| **Memory** | Testing, simulation | Deterministic, synchronized, no I/O |
| **TCP** | Production, LAN | Reliable, standard, proven |
| **Mock** | Unit testing, fault injection | Fine-grained control, hooks |
| **Future: QUIC** | Low-latency, connection multiplexing | Faster than TCP, UDP-based |
| **Future: WebSocket** | Browser clients, HTTP proxies | Web-native, NAT-friendly |
| **Future: UDP** | High-throughput, lossy scenarios | Low overhead, unreliable |

## Implementations

### Memory Transport

**Best for testing and deterministic simulation.**

```go
transport := NewMemoryTransport(DefaultConfig())
listener, _ := transport.Listen(ctx, "server:5000")
conn, _ := transport.Dial(ctx, "server:5000")
```

Characteristics:
- Fully synchronous (no real concurrency)
- Deterministic message ordering
- No network latency
- Perfect for unit tests and algorithmic validation
- All operations succeed unless explicitly closed

### Mock Transport

**Best for unit testing with failure injection.**

```go
mock := NewMockTransport(DefaultConfig())

// Inject failures
mock.SetOnSend(func(msg *Message) error {
    if msg.Seq > 10 {
        return ErrDialFailed
    }
    return nil
})

// Drop specific messages
mock.SetMessageDropper(func(msg *Message) bool {
    return msg.Seq%5 == 0  // Drop every 5th message
})

// Monitor calls
conn := mock.GetConnection("server:5000")
if sendCount := conn.SendCount(); sendCount > 100 {
    // Too many sends
}
```

Capabilities:
- Per-message hooks (global and per-connection)
- Failure injection (send failures, recv failures, dial failures)
- Message dropping/filtering
- Call counting for metrics
- Connection querying and inspection

### TCP Transport

**Best for production distributed systems.**

```go
transport := NewTCPTransport(config)
listener, _ := transport.Listen(ctx, "0.0.0.0:5000")
conn, _ := transport.Dial(ctx, "remote.host:5000")
```

Features:
- Binary message framing (4-byte big-endian length prefix)
- Configurable timeouts (dial, read, write)
- Automatic buffering (64KB buffers by default)
- Connection pooling support
- Graceful shutdown

**Binary Frame Format:**
```
[4-byte length][message data]

Message data structure:
[1-byte from-len][from][1-byte to-len][to]
[8-byte seq][4-byte body-len][body]
[4-byte vc-len][vector-clock-data]
```

## Concurrency Model

### Send Semantics

- **Thread-safe**: Multiple goroutines can safely call Send
- **No ordering guarantee**: Concurrent Sends may reorder
- **Non-blocking**: Returns immediately on successful queue
- **Recommendation**: Use external synchronization for ordered sends

```go
// Safe but order not guaranteed
go conn.Send(ctx, msg1)
go conn.Send(ctx, msg2)

// For ordered sends
mu.Lock()
defer mu.Unlock()
conn.Send(ctx, msg1)
conn.Send(ctx, msg2)
```

### Recv Semantics

- **Thread-safe**: Only one goroutine should call Recv per connection
- **Blocking**: Waits for next message
- **Cancellable**: Respects context cancellation
- **Ordered**: Messages received in order sent

```go
// Good: one receiver per connection
go func(conn Connection) {
    for {
        msg, err := conn.Recv(ctx)
        if err != nil {
            break
        }
        processMessage(msg)
    }
}(serverConn)
```

### Listener Semantics

- **Thread-safe**: Single Accept loop recommended
- **Blocking**: Waits for next connection
- **Cancellable**: Respects context cancellation

```go
listener, _ := transport.Listen(ctx, "0.0.0.0:5000")

for {
    conn, err := listener.Accept(ctx)
    if err != nil {
        break
    }
    go handleConnection(conn)
}
```

## Failure Handling

### Connection Failures

**Send failures** should trigger reconnection logic at the application layer:

```go
func sendWithRetry(conn Connection, msg *Message, maxRetries int) error {
    for attempt := 0; attempt < maxRetries; attempt++ {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        err := conn.Send(ctx, msg)
        cancel()

        if err == nil {
            return nil
        }

        if err == ErrConnectionClosed {
            // Reconnect and retry
            newConn, _ := dialer.Dial(...)
            conn = newConn
            continue
        }

        return err
    }
    return fmt.Errorf("failed after %d retries", maxRetries)
}
```

**Recv failures** indicate connection loss:

```go
for {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    msg, err := conn.Recv(ctx)
    cancel()

    if err == ErrContextCancelled && time.Since(lastMsg) > 30*time.Second {
        // Timeout - reconnect
        return reconnect()
    }

    if err == ErrConnectionClosed {
        return reconnect()
    }

    if err != nil {
        // Other error - retry
        continue
    }

    processMessage(msg)
}
```

### Graceful Shutdown

```go
// Signal shutdown
shutdownChan := make(chan struct{})

// Stop accepting new connections
shutdownChan <- struct{}{}

// Close existing connections gracefully
for _, conn := range activeConnections {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    // Drain any pending messages before closing
    for {
        msg, err := conn.Recv(ctx)
        if err != nil {
            break
        }
        processMessage(msg)
    }
    cancel()
    conn.Close()
}

transport.Close()
```

## Integration with Vector Clocks

### Pattern 1: Clock Embedding

Include marshalled clock data in `Message.VectorClockData`:

```go
import (
    "encoding/json"
    vclock "github.com/dmundt/causalclock"
)

// Send
clock := vclock.NewClock()
clock.Increment("node-1")

clockData, _ := json.Marshal(clock)
msg := &Message{
    From: "node-1",
    To: "node-2",
    Body: appData,
    VectorClockData: clockData,
}
conn.Send(ctx, msg)

// Receive
received, _ := conn.Recv(ctx)
var receivedClock vclock.Clock
json.Unmarshal(received.VectorClockData, &receivedClock)
```

### Pattern 2: Causality Check Before Processing

```go
func processMessage(msg *Message, localClock *vclock.Clock) error {
    if msg.VectorClockData == nil {
        return fmt.Errorf("missing vector clock data")
    }

    // Deserialize sender's clock
    var senderClock vclock.Clock
    json.Unmarshal(msg.VectorClockData, &senderClock)

    // Check causality
    if localClock.HappenedBefore(&senderClock) {
        // Sender has later causality - might be race condition
        return fmt.Errorf("message from future: local %v < sender %v", 
            localClock, senderClock)
    }

    // Safe to process
    localClock.Merge(&senderClock)
    localClock.Increment("node-2")

    return nil
}
```

### Pattern 3: Message with Metadata

```go
msg := &Message{
    From: "node-1",
    To: "node-2",
    Body: appData,
    VectorClockData: clockData,
    Seq: atomic.AddUint64(&msgSeq, 1),
    Timestamp: time.Now(),
    Metadata: map[string]string{
        "version": "1.0",
        "compression": "none",
        "priority": "high",
    },
}
```

## Testing Strategies

### Unit Testing with Memory Transport

```go
func TestEventOrdering(t *testing.T) {
    transport := NewMemoryTransport(DefaultConfig())
    defer transport.Close()

    // Setup nodes A and B
    listener, _ := transport.Listen(context.Background(), "b:5000")
    var connB Connection
    go func() {
        ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
        connB, _ = listener.Accept(ctx)
    }()

    connA, _ := transport.Dial(context.Background(), "b:5000")
    time.Sleep(10 * time.Millisecond)

    // Test: A sends to B, order preserved
    for i := 0; i < 10; i++ {
        msg := &Message{Seq: uint64(i), Body: []byte("data")}
        connA.Send(context.Background(), msg)
    }

    for i := 0; i < 10; i++ {
        ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)
        msg, _ := connB.Recv(ctx)
        if msg.Seq != uint64(i) {
            t.Errorf("Got seq %d, want %d", msg.Seq, i)
        }
    }
}
```

### Fault Injection with Mock Transport

```go
func TestRecoveryFromNetworkPartition(t *testing.T) {
    mock := NewMockTransport(DefaultConfig())
    listener, _ := mock.Listen(context.Background(), "server:5000")

    var serverConn Connection
    go func() {
        ctx, _ := context.WithTimeout(context.Background(), 2*time.Second)
        serverConn, _ = listener.Accept(ctx)
    }()

    clientConn, _ := mock.Dial(context.Background(), "server:5000")
    time.Sleep(10 * time.Millisecond)

    // Send succeeds
    msg1 := &Message{Body: []byte("before")}
    if err := clientConn.Send(context.Background(), msg1); err != nil {
        t.Error(err)
    }

    // Simulate partition
    mockConn := mock.GetConnection("server:5000")
    mockConn.SetFailSend(true)

    // Send fails
    msg2 := &Message{Body: []byte("during")}
    if err := clientConn.Send(context.Background(), msg2); err == nil {
        t.Error("expected send to fail")
    }

    // Recover partition
    mockConn.SetFailSend(false)

    // Send succeeds
    msg3 := &Message{Body: []byte("after")}
    if err := clientConn.Send(context.Background(), msg3); err != nil {
        t.Error(err)
    }
}
```

## Extensibility

### Adding New Transports

Implement three interfaces:

```go
type CustomTransport struct {
    // Implementation fields
}

func (t *CustomTransport) Listen(ctx context.Context, addr string) (Listener, error) {
    // Create listener
    return &customListener{...}, nil
}

func (t *CustomTransport) Dial(ctx context.Context, addr string) (Connection, error) {
    // Create outbound connection
    return &customConnection{...}, nil
}

func (t *CustomTransport) Close() error {
    // Clean up
    return nil
}

// Implement Listener interface
type customListener struct { /* ... */ }
func (l *customListener) Accept(ctx context.Context) (Connection, error) { /* ... */ }
func (l *customListener) Close() error { /* ... */ }
func (l *customListener) Addr() string { /* ... */ }

// Implement Connection interface
type customConnection struct { /* ... */ }
func (c *customConnection) Send(ctx context.Context, msg *Message) error { /* ... */ }
func (c *customConnection) Recv(ctx context.Context) (*Message, error) { /* ... */ }
func (c *customConnection) Close() error { /* ... */ }
func (c *customConnection) LocalAddr() string { /* ... */ }
func (c *customConnection) RemoteAddr() string { /* ... */ }
```

### Protocol Composition

Stack transports for composite behaviors:

```go
// Base TCP transport
baseTcp := NewTCPTransport(config)

// Wrap with compression
compressed := &CompressionWrapper{
    inner: baseTcp,
    compress: gzipCompress,
    decompress: gzipDecompress,
}

// Wrap with encryption
encrypted := &EncryptionWrapper{
    inner: compressed,
    encrypt: aesEncrypt,
    decrypt: aesDecrypt,
}

// Use wrapped transport
listener, _ := encrypted.Listen(ctx, "0.0.0.0:5000")
```

### Connection Pooling

```go
type ConnectionPool struct {
    transport Transport
    mu sync.RWMutex
    pools map[string]chan Connection
}

func (p *ConnectionPool) Get(ctx context.Context, addr string) (Connection, error) {
    p.mu.RLock()
    pool, exists := p.pools[addr]
    p.mu.RUnlock()

    if exists {
        select {
        case conn := <-pool:
            return conn, nil
        default:
        }
    }

    return p.transport.Dial(ctx, addr)
}

func (p *ConnectionPool) Return(addr string, conn Connection) {
    p.mu.RLock()
    pool, exists := p.pools[addr]
    p.mu.RUnlock()

    if !exists {
        return
    }

    select {
    case pool <- conn:
    default:
        conn.Close()
    }
}
```

## Performance Considerations

### Memory Transport
- **Throughput**: ~1M messages/sec (machine dependent)
- **Latency**: Sub-microsecond
- **Memory**: Unbounded queue growth (accumulates all pending messages)
- **Use**: Testing only, not for large workloads

### TCP Transport
- **Throughput**: 10-50K messages/sec (network dependent)
- **Latency**: 1-10ms (network dependent)
- **Memory**: Configurable (default 64KB send/recv buffers)
- **Use**: Production for reliable networks

### Mock Transport
- **Throughput**: ~100K messages/sec (with channels)
- **Latency**: Sub-millisecond
- **Memory**: Configurable per-connection queue
- **Use**: Unit testing with failure injection

## Key Design Goals Achieved

✅ **Minimal Interface** - 3 core interfaces, ~10 methods  
✅ **Transport-Agnostic** - Multiple backends, no tied abstractions  
✅ **No Serialization Assumptions** - Caller controls encoding  
✅ **Deterministic Testing** - Memory transport is fully deterministic  
✅ **Clean Separation** - Transport doesn't know about clock logic  
✅ **Composable** - Easy to stack middlewares (compression, encryption, etc)  
✅ **Context-Driven** - All blocking operations respect cancellation  
✅ **Failure Testable** - Mock transport enables fine-grained fault injection  

## Related Concepts

- **Vector Clock**: Causality tracking (separate `vclock` package)
- **Message Ordering**: Application-level concern with vector clocks
- **Exactly-Once Delivery**: Implement with [seq + ack pattern]
- **Consensus**: Use with Raft/Paxos at application layer
- **Connection Pooling**: Optional wrapper around Transport
