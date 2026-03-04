# Vector Clocks and Version Vectors for Go

[![CI](https://github.com/dmundt/causalclock/actions/workflows/ci.yml/badge.svg)](https://github.com/dmundt/causalclock/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/dmundt/causalclock)](https://github.com/dmundt/causalclock)
[![Go Doc](https://pkg.go.dev/badge/github.com/dmundt/causalclock)](https://pkg.go.dev/github.com/dmundt/causalclock)

Production-ready Go library for distributed systems causal tracking with:
- **Vector Clocks**: Event ordering and happens-before relationships
- **Version Vectors**: Per-object version tracking with Dynamo/Riak semantics
- **Message Framing**: Versioned message layer with pluggable serialization
- **Transport Abstraction**: Optional transport layer (TCP, in-memory, mock)

## Features

- **Pure Logic Core**: No I/O, networking, or dependencies in clock/version packages
- **Deterministic**: Stable iteration order and reproducible behavior
- **Concurrency-Safe**: Thread-safe when used with external locking
- **Minimal Dependencies**: Core has zero deps; optional CBOR for message framing
- **Fully Tested**: 95%+ test coverage with comprehensive edge cases
- **Well-Documented**: Examples and detailed guides for all components

## Installation

```bash
go get github.com/dmundt/causalclock
```

## Quick Reference

| Package | Purpose | Use When | Dependencies | Coverage |
|---------|---------|----------|--------------|----------|
| **clock** | Event causality tracking | Distributed protocols, message ordering | None | 95.3% |
| **version** | Object version tracking | Multi-master replication, conflict detection | None | 94.0% |
| **message** | Message framing + serialization | Building distributed apps, need wire format | cbor | 91.7% |
| **transport** | Network abstraction | Need transport layer, deterministic testing | None | 42.0% |

**Typical Combinations**:
- **Just clocks**: `import "github.com/dmundt/causalclock/clock"`
- **Clocks + messaging**: Add `message` package
- **Complete system**: Use all packages with `transport`
- **Testing**: Use `transport.MemoryTransport` for deterministic tests

## Quick Start

### Vector Clocks (Event Ordering)

```go
package main

import (
    "fmt"
    "github.com/dmundt/causalclock"
)

func main() {
    // Create clocks for two nodes
    alice := vclock.NewClock()
    bob := vclock.NewClock()
    
    // Alice does work
    alice.Increment("alice")
    alice.Increment("alice")
    
    // Bob receives Alice's clock and does work
    bob.Merge(alice)
    bob.Increment("bob")
    
    // Check causality
    fmt.Println(bob.HappenedAfter(alice)) // true
}
```

### Version Vectors (Object Versioning)

```go
package main

import (
    "fmt"
    "github.com/dmundt/causalclock"
)

func main() {
    // Track versions for an object across replicas
    version := vclock.NewVersionVector()
    
    // Replica A updates
    version.Increment("A")
    
    // Replica B receives and updates
    versionB := version.Copy()
    versionB.Increment("B")
    
    // Detect causality
    fmt.Println(version.HappenedBefore(versionB)) // true
}
```

## Architecture Overview

This library provides four distinct layers with clear separation of concerns:

```
┌─────────────────────────────────────────────────────────────┐
│                    Application Layer                         │
│  (Your distributed system: database, cache, queue, etc.)    │
└─────────────────────────────────────────────────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌──────────────┐    ┌──────────────────┐    ┌──────────────┐
│ Vector Clock │    │ Version Vector   │    │   Message    │
│   (events)   │    │   (objects)      │    │   Framing    │
│              │    │                  │    │ (optional)   │
│  Pure Logic  │    │   Pure Logic     │    │ Serializers  │
│  No I/O      │    │   No I/O         │    │ JSON/CBOR    │
└──────────────┘    └──────────────────┘    └──────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │    Transport     │
                    │   Abstraction    │
                    │   (optional)     │
                    └──────────────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
        ▼                     ▼                     ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│     TCP      │    │   In-Memory  │    │     Mock     │
│  Transport   │    │  (testing)   │    │   (testing)  │
└──────────────┘    └──────────────┘    └──────────────┘
```

### Layer Responsibilities

| Layer | Purpose | Dependencies | I/O |
|-------|---------|--------------|-----|
| **clock** | Vector clocks for event causality | None | No |
| **version** | Version vectors for object versioning | None | No |
| **message** | Message framing with clocks embedded | clock, cbor | No |
| **transport** | Network abstraction (optional) | None | Yes |

### Design Principles

1. **Core is Pure**: `clock` and `version` packages have zero I/O
2. **Optional Layers**: Use only what you need (clocks without transport, etc.)
3. **Pluggable**: Bring your own serialization, transport, storage
4. **Testable**: In-memory transport for deterministic testing

## Understanding Causality

### Causal Relations Diagram

Vector clocks track four possible relationships between events:

```
Event A                    Event B
  │                          │
  │  A happened-before B    │
  │  (A → B)                │
  ├─────────────────────────►
  │                          │
  │                          │
  
  │                          │
  │  B happened-before A    │
  │  (B → A)                │
  ◄─────────────────────────┤
  │                          │
  │                          │
  
  │                          │
  │  A and B are CONCURRENT │
  │  (A ∥ B)                │
  │         ╱╲               │
  │        ╱  ╲              │
  │       ╱    ╲             │
  
  │                          │
  │  A equals B             │
  │  (A = B)                │
  │  ════════════════════   │
```

### Causal History Example

```
Time flows downward ↓

Node A          Node B          Node C
  │               │               │
  │ e1: {A:1}     │               │
  │ "update x"    │               │
  │               │               │
  ├──────msg────►│               │
  │               │ e2: {A:1,B:1} │
  │               │ "read x"      │
  │               │               │
  │               ├──────msg────►│
  │               │               │ e3: {A:1,B:1,C:1}
  │               │               │ "process x"
  │ e4: {A:2}     │               │
  │ "delete x"    │               │
  │               │               │
  │◄──────────────┼───────msg─────┤
  │               │               │
  │ e5: {A:2,B:1,C:1}            │
  │ CONFLICT: e4 ∥ e3            │
  │ (concurrent delete/process)  │

Causal relationships:
• e1 → e2  (happened-before)
• e2 → e3  (happened-before)
• e1 → e3  (transitive)
• e4 ∥ e3  (concurrent - CONFLICT!)
```

### Vector Clock States

```
Clock 1: {A:5, B:3, C:2}     Clock 2: {A:5, B:4, C:2}
         ┌───┬───┬───┐                ┌───┬───┬───┐
         │ 5 │ 3 │ 2 │                │ 5 │ 4 │ 2 │
         └───┴───┴───┘                └───┴───┴───┘
              │                              ▲
              └──────── Before ──────────────┘
         (Clock 1 happened-before Clock 2)


Clock 1: {A:5, B:3, C:2}     Clock 2: {A:4, B:5, C:1}
         ┌───┬───┬───┐                ┌───┬───┬───┐
         │ 5 │ 3 │ 2 │                │ 4 │ 5 │ 1 │
         └───┴───┴───┘                └───┴───┴───┘
              │                              │
              └────── Concurrent ────────────┘
               (Neither ≤ nor ≥ - CONFLICT!)
```

## When to Use What

### Decision Guide

```
┌─────────────────────────────────────────────┐
│ What are you tracking?                      │
└─────────────────────────────────────────────┘
              │
      ┌───────┴────────┐
      │                │
      ▼                ▼
┌──────────┐    ┌─────────────┐
│  Events  │    │   Objects   │
│  (when)  │    │   (what)    │
└──────────┘    └─────────────┘
      │                │
      ▼                ▼
 Vector Clock    Version Vector
      │                │
      │                │
      ├────────────────┤
      │                │
      ▼                ▼
┌────────────────────────────┐
│ Need messaging?            │
└────────────────────────────┘
      │
      ├─── Yes ──► message.Message
      │            + Serializer
      │
      └─── No ───► Use clocks directly
                  
┌────────────────────────────┐
│ Need transport?            │
└────────────────────────────┘
      │
      ├─── Production ──► TCP Transport
      │
      ├─── Testing ────► Memory Transport
      │
      └─── Mocking ────► Mock Transport
```

### Use Vector Clocks When

✓ Tracking **causality between events**  
✓ Implementing **distributed protocols** (2PC, Paxos, Raft)  
✓ Detecting **race conditions** in concurrent execution  
✓ Building **message queues** with causal ordering  
✓ Creating **distributed debuggers** or **replay systems**  
✓ Implementing **snapshot isolation** algorithms  

**Example Use Cases:**
- Distributed tracing (event causality)
- Lamport clocks replacement (better conflict detection)
- Happens-before tracking in CSP-style systems
- Causal broadcast protocols

### Use Version Vectors When

✓ Tracking **per-object versions** across replicas  
✓ Implementing **multi-master replication**  
✓ Building **eventually consistent** key-value stores  
✓ Detecting **write conflicts** (concurrent updates)  
✓ Implementing **shopping cart** merge logic  
✓ Building **collaborative editing** systems  

**Example Use Cases:**
- Dynamo/Riak-style databases
- CRDTs (Conflict-free Replicated Data Types)
- Document version control
- Session management with multiple writers

### Use Message Framing When

✓ Need **versioned message format** for protocol evolution  
✓ Want **pluggable serialization** (JSON, CBOR, Protobuf)  
✓ Handling **untrusted input** (validation, size limits)  
✓ Building **distributed applications** with message passing  
✓ Need **automatic clock embedding** in messages  

### Use Transport Layer When

✓ Building **complete distributed system** (not just library)  
✓ Need **abstraction over TCP/QUIC/memory**  
✓ Want **deterministic testing** with in-memory transport  
✓ Implementing **connection pooling** or **retry logic**  

**Skip Transport Layer When:**
- Using existing RPC framework (gRPC, Thrift)
- Already have networking layer
- Only need causality tracking

## Quick Start

### Vector Clocks (Event Ordering)

```go
package main

import (
    "fmt"
    "github.com/dmundt/causalclock/clock"
)

func main() {
    // Create clocks for two nodes
    alice := clock.NewClock()
    bob := clock.NewClock()
    
    // Alice does work
    alice.Increment("alice")
    alice.Increment("alice")
    
    // Bob receives Alice's clock and does work
    bob.Merge(alice)
    bob.Increment("bob")
    
    // Check causality
    fmt.Println(bob.HappenedAfter(alice)) // true
}
```

### Version Vectors (Object Versioning)

```go
package main

import (
    "fmt"
    "github.com/dmundt/causalclock/version"
)

func main() {
    // Track versions for an object across replicas
    version := vvector.NewVersionVector()
    
    // Replica A updates
    version.Increment("A")
    
    // Replica B receives and updates
    versionB := version.Copy()
    versionB.Increment("B")
    
    // Detect causality
    fmt.Println(version.HappenedBefore(versionB)) // true
}

### Vector Clocks

**Core Operations**
- **`NewClock(nodes ...NodeID) *Clock`** - Create a new vector clock
- **`Increment(node NodeID) int64`** - Increment and return new value
- **`Get(node NodeID) int64`** - Read a node's clock value
- **`Set(node NodeID, value int64)`** - Set a node's clock value
- **`Merge(other *Clock)`** - Merge two clocks (element-wise max)
- **`Copy() *Clock`** - Create a deep copy

**Comparison Operations**
- **`Compare(other *Clock) Comparison`** - Full comparison (returns Equal, Before, After, Concurrent)
- **`Equal(other *Clock) bool`** - Test for equality
- **`HappenedBefore(other *Clock) bool`** - Test for causality
- **`HappenedAfter(other *Clock) bool`** - Test for causality
- **`Concurrent(other *Clock) bool`** - Test for concurrency
- **`Ancestor(other *Clock) bool`** - Happened-before or equal
- **`Descendant(other *Clock) bool`** - Happened-after or equal

**Utility Operations**
- **`Nodes() []NodeID`** - Get sorted list of nodes
- **`Len() int`** - Number of tracked nodes
- **`IsEmpty() bool`** - Check if all values are zero
- **`String() string`** - Human-readable representation
- **`ParseClock(s string) (*Clock, error)`** - Parse from string

### Version Vectors

**Core Operations**
- **`NewVersionVector(replicas ...ReplicaID) *VersionVector`** - Create a new version vector
- **`Increment(replica ReplicaID) uint64`** - Increment replica counter
- **`Get(replica ReplicaID) uint64`** - Read a replica's version
- **`Set(replica ReplicaID, version uint64)`** - Set a replica's version
- **`Merge(other *VersionVector)`** - Merge with Dynamo/Riak semantics (element-wise max)
- **`Copy() *VersionVector`** - Create a deep copy

**Comparison Operations**
- **`Compare(other *VersionVector) Comparison`** - Full comparison
- **`Equal(other *VersionVector) bool`** - Test equality
- **`HappenedBefore(other *VersionVector) bool`** - Ancestor check
- **`HappenedAfter(other *VersionVector) bool`** - Descendant check
- **`Concurrent(other *VersionVector) bool`** - Conflict detection
- **`Descends(other *VersionVector) bool`** - Descendant or equal
- **`Dominates(other *VersionVector) bool`** - Strictly newer
- **`IsDominatedBy(other *VersionVector) bool`** - Strictly older

**Utility Operations**
- **`Replicas() []ReplicaID`** - Get sorted list of replicas
- **`Len() int`** - Number of tracked replicas
- **`IsEmpty() bool`** - Check if all versions are zero
- **`String() string`** - Human-readable representation

## Integration Examples

### Message Framing Integration

The `message` package provides versioned message framing with embedded vector clocks:

```go
package main

import (
    "log"
    "github.com/dmundt/causalclock/clock"
    "github.com/dmundt/causalclock/message"
)

func main() {
    // Node A creates message
    clockA := clock.NewClock()
    clockA.Increment("nodeA")
    
    msg, err := message.NewMessage("nodeA", clockA, []byte("Hello"))
    if err != nil {
        log.Fatal(err)
    }
    
    // Add metadata for routing/tracing
    msg.WithMetadata("trace_id", "abc123").
        WithMetadata("priority", "high")
    
    // Serialize with JSON (human-readable)
    jsonSer := &message.JSONSerializer{}
    wireData, _ := jsonSer.Marshal(msg)
    
    // Node B receives and deserializes
    received, _ := jsonSer.Unmarshal(wireData)
    
    // Merge clocks for causality tracking
    clockB := clock.NewClock()
    clockB.Merge(received.Clock)
    clockB.Increment("nodeB")
    
    log.Printf("Node B processed message from %s", received.SenderID)
}
```

### CBOR vs JSON Serialization

Choose serializer based on requirements:

```go
// JSON: Human-readable, debugging-friendly
jsonSer := &message.JSONSerializer{}
jsonData, _ := jsonSer.Marshal(msg)

// CBOR: Binary, compact (50-70% of JSON size)
cborSer, _ := message.NewCBORSerializer()
cborData, _ := cborSer.Marshal(msg)

log.Printf("JSON: %d bytes", len(jsonData))  // ~200 bytes
log.Printf("CBOR: %d bytes", len(cborData))  // ~120 bytes
```

### Transport Integration: In-Memory (Testing)

Perfect for deterministic testing of distributed algorithms:

```go
package main

import (
    "context"
    "testing"
    "github.com/dmundt/causalclock/clock"
    "github.com/dmundt/causalclock/message"
    "github.com/dmundt/causalclock/transport"
)

func TestDistributedProtocol(t *testing.T) {
    // Create in-memory transport
    tr := transport.NewMemoryTransport(transport.DefaultConfig())
    defer tr.Close()
    
    ctx := context.Background()
    
    // Node A: Start listener
    listenerA, _ := tr.Listen(ctx, "nodeA")
    go func() {
        conn, _ := listenerA.Accept(ctx)
        defer conn.Close()
        
        // Receive message
        msg, _ := conn.Recv(ctx)
        t.Logf("Received from %s", msg.From)
    }()
    
    // Node B: Connect and send
    connB, _ := tr.Dial(ctx, "nodeA")
    defer connB.Close()
    
    clk := clock.NewClock()
    clk.Increment("nodeB")
    
    msg := &transport.Message{
        From: "nodeB",
        To:   "nodeA",
        Body: []byte("test message"),
    }
    
    connB.Send(ctx, msg)
}
```

### Transport Integration: TCP (Production)

For production systems needing real network communication:

```go
package main

import (
    "context"
    "log"
    "github.com/dmundt/causalclock/transport"
)

func main() {
    config := transport.DefaultConfig()
    config.NodeID = "server1"
    
    tr := transport.NewTCPTransport(config)
    defer tr.Close()
    
    ctx := context.Background()
    
    // Server: Listen on TCP
    listener, err := tr.Listen(ctx, "localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    
    go func() {
        for {
            conn, err := listener.Accept(ctx)
            if err != nil {
                return
            }
            
            go handleConnection(conn)
        }
    }()
    
    // Client: Connect via TCP
    conn, err := tr.Dial(ctx, "localhost:8080")
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()
    
    msg := &transport.Message{
        From: "client1",
        To:   "server1",
        Body: []byte("Hello TCP"),
    }
    
    conn.Send(ctx, msg)
}

func handleConnection(conn transport.Connection) {
    defer conn.Close()
    ctx := context.Background()
    
    for {
        msg, err := conn.Recv(ctx)
        if err != nil {
            return
        }
        
        log.Printf("From %s: %s", msg.From, string(msg.Body))
    }
}
```

### Complete Distributed System Example

Combining all layers for a complete causal broadcast system:

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    
    "github.com/dmundt/causalclock/clock"
    "github.com/dmundt/causalclock/message"
    "github.com/dmundt/causalclock/transport"
)

type Node struct {
    id        string
    clock     *clock.Clock
    transport transport.Transport
    peers     []string
}

func NewNode(id string, tr transport.Transport, peers []string) *Node {
    return &Node{
        id:        id,
        clock:     clock.NewClock(),
        transport: tr,
        peers:     peers,
    }
}

func (n *Node) Broadcast(ctx context.Context, data []byte) error {
    // Increment local clock
    n.clock.Increment(clock.NodeID(n.id))
    
    // Create message with embedded clock
    msg, err := message.NewMessage(n.id, n.clock, data)
    if err != nil {
        return err
    }
    
    // Serialize
    ser := &message.JSONSerializer{}
    wireData, err := ser.Marshal(msg)
    if err != nil {
        return err
    }
    
    // Send to all peers
    for _, peer := range n.peers {
        conn, err := n.transport.Dial(ctx, peer)
        if err != nil {
            log.Printf("Failed to dial %s: %v", peer, err)
            continue
        }
        
        transportMsg := &transport.Message{
            From: n.id,
            To:   peer,
            Body: wireData,
        }
        
        if err := conn.Send(ctx, transportMsg); err != nil {
            log.Printf("Failed to send to %s: %v", peer, err)
        }
        
        conn.Close()
    }
    
    return nil
}

func (n *Node) Receive(ctx context.Context, conn transport.Connection) {
    for {
        // Receive transport message
        transportMsg, err := conn.Recv(ctx)
        if err != nil {
            return
        }
        
        // Deserialize application message
        ser := &message.JSONSerializer{}
        appMsg, err := ser.Unmarshal(transportMsg.Body)
        if err != nil {
            log.Printf("Failed to unmarshal: %v", err)
            continue
        }
        
        // Merge clocks (causal tracking)
        n.clock.Merge(appMsg.Clock)
        n.clock.Increment(clock.NodeID(n.id))
        
        // Process message
        log.Printf("Node %s received from %s: %s (clock: %s)",
            n.id, appMsg.SenderID, string(appMsg.Payload), n.clock.String())
    }
}

func main() {
    ctx := context.Background()
    
    // Create in-memory transport for testing
    tr := transport.NewMemoryTransport(transport.DefaultConfig())
    defer tr.Close()
    
    // Create three nodes
    nodeA := NewNode("A", tr, []string{"B", "C"})
    nodeB := NewNode("B", tr, []string{"A", "C"})
    nodeC := NewNode("C", tr, []string{"A", "B"})
    
    // Start listeners
    for _, node := range []*Node{nodeA, nodeB, nodeC} {
        listener, _ := tr.Listen(ctx, node.id)
        go func(n *Node, l transport.Listener) {
            for {
                conn, err := l.Accept(ctx)
                if err != nil {
                    return
                }
                go n.Receive(ctx, conn)
            }
        }(node, listener)
    }
    
    // Broadcast from node A
    nodeA.Broadcast(ctx, []byte("Event from A"))
    
    // Allow time for message propagation in real system
    // In production, use proper synchronization
}
```

### QUIC Transport Integration

While not included in this library, you can integrate with QUIC:

```go
// Pseudocode for QUIC integration
type QUICTransport struct {
    config transport.TransportConfig
    // ... QUIC-specific fields
}

func (t *QUICTransport) Dial(ctx context.Context, addr string) (transport.Connection, error) {
    // Use github.com/quic-go/quic-go
    conn, err := quic.DialAddr(ctx, addr, &quic.Config{})
    if err != nil {
        return nil, err
    }
    
    stream, err := conn.OpenStreamSync(ctx)
    if err != nil {
        return nil, err
    }
    
    return &quicConnection{stream: stream}, nil
}

// Implement transport.Connection interface wrapping QUIC stream
```

### Custom Transport Example

Implement your own transport for specialized needs:

```go
package main

import (
    "context"
    "github.com/dmundt/causalclock/transport"
)

// RedisTransport uses Redis pub/sub as transport
type RedisTransport struct {
    config transport.TransportConfig
    client *redis.Client
}

func (t *RedisTransport) Listen(ctx context.Context, addr string) (transport.Listener, error) {
    return &redisListener{
        channel: addr,
        pubsub:  t.client.Subscribe(ctx, addr),
    }, nil
}

func (t *RedisTransport) Dial(ctx context.Context, addr string) (transport.Connection, error) {
    return &redisConnection{
        channel: addr,
        client:  t.client,
    }, nil
}

// Implement transport.Connection and transport.Listener interfaces
// using Redis pub/sub primitives
```

## Design Decisions

### Vector Clocks vs Version Vectors

**When to use Vector Clocks:**
- Tracking causality between events in a distributed system
- Message ordering in distributed protocols
- Detecting race conditions
- General happens-before relationship tracking

**When to use Version Vectors:**
- Per-object version tracking (Dynamo, Riak, Cassandra style)
- Conflict detection in eventually consistent systems
- Multi-master replication
- Shopping cart merging, collaborative editing

**Key Differences:**
- Vector Clocks use `int64` (signed, for flexibility)
- Version Vectors use `uint64` (unsigned, versions are always positive)
- Version Vectors follow Dynamo/Riak semantics explicitly
- Terminology: NodeID vs ReplicaID reflects different use cases

### 1. NodeID/ReplicaID as String

**Decision**: Use `string` for identifiers.

**Rationale**:
- Flexibility: Supports UUIDs, hostnames, IP addresses, etc.
- No artificial constraints on ID format
- Minimal memory overhead in practice
- Go's string type is immutable and efficient

**Tradeoff**: Slightly less type-safe than custom types, but gains flexibility.

### 2. Map-Based Storage

**Decision**: Internal `map[ID]counter` for both implementations.

**Rationale**:
- O(1) lookups and updates
- Sparse representation (only tracks seen nodes/replicas)
- Natural fit for distributed systems (dynamic membership)
- Memory-efficient for typical cluster sizes

**Tradeoff**: Slightly more overhead than arrays for small fixed sets, but more flexible.

### 3. Counter Types

**Decision**: 
- Vector Clocks use `int64` (signed)
- Version Vectors use `uint64` (unsigned)

**Rationale**:
- **int64 for Vector Clocks**: 
  - Extremely large range: 9.2 quintillion increments
  - Signed allows negative values (useful for debugging/testing)
  - Fixed size = predictable performance
  - Overflow practically impossible (1M increments/sec = 292K years)
  
- **uint64 for Version Vectors**:
  - Version numbers are conceptually unsigned (never negative)
  - Follows Dynamo/Riak conventions
  - Slightly larger positive range (18.4 quintillion)
  - Makes semantic intent clear (versions start at 0 or 1)

**Tradeoff**: Cannot detect overflow, but overflow is practically impossible.

### 4. Internal Map Not Exported

**Decision**: The internal map is private.

**Rationale**:
- Prevents direct mutation bypassing API contracts
- Allows implementation changes without breaking API
- Enforces Copy-on-Write semantics where needed
- Enables invariant validation if needed later

**Tradeoff**: Requires copying for snapshots, but Copy() is explicit and efficient.

### 5. Element-Wise Maximum in Merge

**Decision**: Both implementations use element-wise maximum merge.

**Rationale**:
- Standard algorithm for both vector clocks and version vectors
- Preserves causality (happens-before relationships)
- Commutative and associative (order doesn't matter)
- Idempotent (safe to merge multiple times)
- Matches Dynamo/Riak semantics for version vectors

**Alternative Considered**: Copy-on-write merge returning new structure. Rejected for performance and mutation clarity.

### 6. Comparison Semantics

**Decision**: Four explicit comparison results: Equal, Before, After, Concurrent.

**Rationale**:
- Matches distributed systems literature
- Concurrent (conflict) detection is critical for eventual consistency
- Single comparison operation more efficient than multiple calls
- Impossible to have contradictory states
- Clear API semantics

**Implementation**:
- Before: v1[i] ≤ v2[i] for all i, and v1[j] < v2[j] for at least one j
- After: v1[i] ≥ v2[i] for all i, and v1[j] > v2[j] for at least one j  
- Equal: v1[i] = v2[i] for all i
- Concurrent: Neither Before nor After (indicates conflict)

### 7. Deterministic Iteration Order

**Decision**: Always sort IDs before iteration/serialization.

**Rationale**:
- Reproducible output for testing
- Consistent serialization
- Debuggability (same state = same string)
- Prevents Go map iteration non-determinism from leaking

**Tradeoff**: O(n log n) sorting cost, negligible for typical sizes (<1000 nodes/replicas).

### 8. No Internal Locking

**Decision**: Neither implementation is internally synchronized.

**Rationale**:
- Composability: Users choose locking granularity
- Performance: No lock overhead when not needed
- Flexibility: Supports lock-free algorithms with external coordination
- Testability: Deterministic without lock ordering issues
- Separation of concerns: Data structure vs synchronization

**Tradeoff**: Requires external locking for concurrent access, but this is more flexible.

### 10. Zero Value is Valid

**Decision**: `var c Clock` and `var vv VersionVector` are valid empty structures.

**Rationale**:
- Idiomatic Go (zero values should be useful)
- Simplifies initialization
- No "constructed vs. unconstructed" state to track

**Tradeoff**: None significant.

### 11. Immutability via Copy

**Decision**: Provide `Copy()` instead of making structures immutable.

**Rationale**:
- Performance: Avoids unnecessary copies for every operation
- Flexibility: Users choose when to snapshot
- Clarity: Mutations are explicit (`Increment`, `Set`, `Merge`)
- Go idiom: Value semantics via explicit copy

**Alternative Considered**: Persistent data structures. Rejected for complexity and performance.

## Version Vector Design Notes

### Dynamo/Riak Semantics

Version vectors in this implementation follow the semantics used by Amazon Dynamo and Riak:

1. **Per-Object Versioning**: Each object has its own version vector
2. **Element-Wise Maximum Merge**: When merging, take max(v1[i], v2[i]) for each replica
3. **Conflict Detection**: Concurrent versions indicate conflicts requiring resolution
4. **Sibling Resolution**: Application handles conflicting versions (last-write-wins, merge, manual)

### Comparison Relations

Version vectors support all four causal relations:

- **Equal**: Identical versions (same object state)
- **Before**: v1 is an ancestor of v2 (v1 happened-before v2)
- **After**: v1 is a descendant of v2 (v1 happened-after v2)
- **Concurrent**: Neither happened-before (conflict/divergent versions)

These match the theoretical foundations from:
- Lamport (1978): "Time, Clocks, and the Ordering of Events"
- Fidge (1988): "Timestamps in Message-Passing Systems"
- Mattern (1988): "Virtual Time and Global States"

### Use Cases

**Version Vectors are ideal for:**
- Shopping carts (Amazon, Riak)
- Session management
- Document versioning (collaborative editing)
- Multi-master databases (Cassandra, Riak, Voldemort)
- Conflict-free replicated data types (CRDTs)
- Eventually consistent key-value stores

**Not suitable for:**
- Single-master replication (use simple version numbers)
- Systems requiring total ordering (use logical clocks or timestamps)
- Real-time constraints (version vectors don't capture wall-clock time)

## Edge Cases and Handling

### 1. Empty Structures

**Vector Clocks**:
```go
c1 := vclock.NewClock()
c2 := vclock.NewClock()
fmt.Println(c1.Equal(c2)) // true
```

**Version Vectors**:
```go
vv1 := vclock.NewVersionVector()
vv2 := vclock.NewVersionVector()
fmt.Println(vv1.Equal(vv2)) // true
```

**Behavior**:
- Empty compares `Equal` to another empty
- Empty `Before` any non-empty with positive values
- Merging with empty is a no-op

### 2. Nil Safety

**Behavior**:
- Nil methods don't panic
- Nil treated as empty clock for comparison
- `nil.Copy()` returns new empty clock

**Example**:
```go
var c *vclock.Clock
fmt.Println(c.Get("node1"))    // 0
fmt.Println(c.Nodes())         // []
fmt.Println(c.Compare(nil))    // Equal
```

### 3. Non-Existent Nodes

**Behavior**:
- `Get(missing)` returns 0 (no entry created)
- `Increment(missing)` initializes to 1
- `Set(missing, value)` creates entry

**Example**:
```go
c := vclock.NewClock()
fmt.Println(c.Get("missing")) // 0, no entry
fmt.Println(c.Len())          // 0
c.Increment("missing")
fmt.Println(c.Len())          // 1
```

### 4. Zero Values

**Behavior**:
- `Set(node, 0)` keeps the entry (doesn't remove it)
- Clock with all zeros is considered empty (`IsEmpty() == true`)
- Zero entries participate in comparisons

**Rationale**: Explicit presence tracking. A node set to 0 is different from a node never seen.

**Example**:
```go
c := vclock.NewClock()
c.Set("node1", 0)
fmt.Println(c.Len())      // 1 (entry exists)
fmt.Println(c.IsEmpty())  // true (all values zero)
```

### 5. Negative Values

**Behavior**:
- `Set(node, -5)` is allowed (not recommended)
- Negative values participate in comparisons normally
- No special handling or validation

**Rationale**: Keep implementation simple; clients can validate if needed.

**Example**:
```go
c := vclock.NewClock()
c.Set("node1", -5)
fmt.Println(c.Get("node1")) // -5
```

### 6. Disjoint Node Sets

**Behavior**:
- Clocks with different nodes compare correctly
- Missing nodes treated as 0 in comparisons
- Merge combines node sets

**Example**:
```go
c1 := vclock.NewClock()
c1.Set("alice", 5)

c2 := vclock.NewClock()
c2.Set("bob", 5)

fmt.Println(c1.Concurrent(c2)) // true (disjoint sets)
```

### 7. Large Clock Values

**Behavior**:
- int64 max = 9,223,372,036,854,775,807
- No overflow detection
- Wrapping to negative on overflow (standard Go behavior)

**Mitigation**: Practically impossible to overflow in real systems.

### 8. Concurrent Updates Detection

**Behavior**:
- Two clocks where neither is <= the other are concurrent
- Indicates conflict in distributed system
- Application decides resolution strategy

**Example**:
```go
c1 := vclock.NewClock()
c1.Set("node1", 5)
c1.Set("node2", 2)

c2 := vclock.NewClock()
c2.Set("node1", 2)
c2.Set("node2", 5)

if c1.Concurrent(c2) {
    // Handle conflict (e.g., merge, last-write-wins, prompt user)
}
```

### 9. String Parsing

**Behavior**:
- `ParseClock` is inverse of `String()`
- Whitespace tolerant
- Returns error on invalid format
- **Not for production serialization** (use protocol buffers, JSON, etc.)

**Example**:
```go
c, err := vclock.ParseClock("{alice:5, bob:3}")
// Round-trip: ParseClock(c.String()) == c
```

### 10. Comparison Symmetry

**Behavior**:
- `c1.Compare(c2) == Before` ⟺ `c2.Compare(c1) == After`
- `Equal` and `Concurrent` are symmetric

**Verified**: All comparison tests check reverse comparison.

## Usage Patterns

### Message Passing Protocol

```go
// Sender
sender := vclock.NewClock()
sender.Increment("sender")
message := Message{
    Data:  payload,
    Clock: sender.Copy(), // Attach clock to message
}

// Receiver
receiver := vclock.NewClock()
func handleMessage(msg Message) {
    receiver.Merge(msg.Clock)   // Merge sender's clock
    receiver.Increment("receiver") // Increment own clock
    // Process message...
}
```

### Conflict Detection

```go
func detectConflict(c1, c2 *vclock.Clock) bool {
    return c1.Concurrent(c2)
}

func resolveConflict(c1, c2 *vclock.Clock) *vclock.Clock {
    merged := c1.Copy()
    merged.Merge(c2)
    // Apply application-specific resolution
    return merged
}
```

### External Locking for Concurrency

```go
type SafeClock struct {
    mu    sync.RWMutex
    clock *vclock.Clock
}

func (s *SafeClock) Increment(node vclock.NodeID) int64 {
    s.mu.Lock()
    defer s.mu.Unlock()
    return s.clock.Increment(node)
}

func (s *SafeClock) Snapshot() *vclock.Clock {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.clock.Copy()
}
```

### Event Ordering

```go
type Event struct {
    ID    string
    Data  interface{}
    Clock *vclock.Clock
}

func orderEvents(e1, e2 Event) string {
    switch e1.Clock.Compare(e2.Clock) {
    case vclock.BeforeCmp:
        return "e1 -> e2"
    case vclock.AfterCmp:
        return "e2 -> e1"
    case vclock.ConcurrentCmp:
        return "e1 || e2"
    case vclock.EqualCmp:
        return "e1 == e2"
    }
    return "unknown"
}
```

## Performance Characteristics

| Operation | Time Complexity | Space Complexity | Notes |
|-----------|----------------|------------------|-------|
| `NewClock()` | O(n) | O(n) | n = number of initial nodes |
| `Increment()` | O(1) | O(1) | Amortized for map growth |
| `Get()` | O(1) | O(1) | Map lookup |
| `Set()` | O(1) | O(1) | Amortized for map growth |
| `Merge()` | O(m) | O(k) | m = other.Len(), k = new nodes |
| `Compare()` | O(n+m) | O(n+m) | n,m = Len() of clocks |
| `Copy()` | O(n) | O(n) | n = Len() |
| `Nodes()` | O(n log n) | O(n) | Sorting for determinism |
| `String()` | O(n log n) | O(n) | Sorting + string building |

**Typical Node Counts**: 3-100 nodes  
**Memory**: ~40 bytes + 16 bytes per node (map overhead)

## Testing

### Running Tests

Run all tests across all packages:
```bash
go test ./...
```

Run with verbose output:
```bash
go test -v ./...
```

Run with coverage:
```bash
go test -cover ./...
```

Output:
```
ok  github.com/dmundt/causalclock/clock     coverage: 95.3%
ok  github.com/dmundt/causalclock/version   coverage: 94.0%
ok  github.com/dmundt/causalclock/message   coverage: 91.7%
ok  github.com/dmundt/causalclock/transport coverage: 42.0%
```

Generate HTML coverage report:
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Running Benchmarks

Run all benchmarks:
```bash
go test -bench=. -benchmem ./...
```

Run specific package benchmarks:
```bash
go test -bench=. -benchmem ./clock
go test -bench=. -benchmem ./version
go test -bench=. -benchmem ./transport
```

### Running Examples

Run example tests:
```bash
go test -run=Example ./...
```

Run specific example:
```bash
go test -run=Example_basicMessage ./message
```

### Test Organization

Each package includes:
- **Unit tests**: Comprehensive functionality testing
- **Example tests**: Executable documentation
- **Benchmarks**: Performance testing at scale

**Test Files**:
- `clock/clock_test.go` - 584 lines, 95.3% coverage
- `clock/example_test.go` - 302 lines, executable examples
- `version/vector_test.go` - 611 lines, 94.0% coverage
- `version/example_test.go` - 257 lines, executable examples
- `message/message_test.go` - 574 lines, 91.7% coverage
- `message/example_test.go` - 179 lines, usage examples
- `transport/transport_test.go` - 42.0% coverage
- `transport/example_test.go` - Integration examples

## Thread Safety

**This package does NOT provide internal synchronization.** Clocks are safe for concurrent reads with no concurrent writes, but concurrent writes require external locking.

### Correct Concurrent Usage

```go
var mu sync.Mutex
clock := vclock.NewClock()

// Concurrent reads (no writes) - SAFE without lock
value := clock.Get("node1")

// Concurrent writes - REQUIRES lock
mu.Lock()
clock.Increment("node1")
mu.Unlock()

// Atomic read-modify-write - REQUIRES lock
mu.Lock()
snapshot := clock.Copy()
snapshot.Merge(otherClock)
clock = snapshot
mu.Unlock()
```

### Why No Internal Locks?

1. **Performance**: Avoids lock overhead when not needed
2. **Composability**: Lets callers batch operations atomically
3. **Flexibility**: Supports custom synchronization strategies
4. **Testability**: Deterministic without lock contention

## Alternatives Considered

### 1. Hybrid Logical Clocks (HLC)

**Tradeoff**: HLCs provide clock-like timestamps but require wall-clock synchronization. Vector clocks are pure logical time.

**Use Vector Clocks When**: You need causality without clock sync, or have many concurrent writes.

**Use HLC When**: You need human-readable timestamps and bounded clock size.

### 2. Dotted Version Vectors (DVV)

**Tradeoff**: DVVs handle concurrent writes more precisely but are more complex.

**Use Version Vectors When**: Standard Dynamo/Riak semantics are sufficient.

**Use DVV When**: You need precise per-key causality in key-value stores with complex write patterns.

### 3. Logical Timestamps

**Tradeoff**: Simpler but only provide total ordering, not causality.

**Use Version Vectors When**: You need to detect conflicts and causality.

**Use Logical Timestamps When**: Simple ordering is sufficient (single writer, append-only logs).

## Version Vector Examples

### Shopping Cart (Dynamo/Riak)

```go
type CartItem struct {
    ProductID string
    Quantity  int
}

type ShoppingCart struct {
    UserID  string
    Items   []CartItem
    Version *vclock.VersionVector
}

// Add item to cart at replica A
cart := &ShoppingCart{
    UserID:  "user123",
    Version: vclock.NewVersionVector(),
}
cart.Version.Increment("replicaA")

// Concurrent update at replica B
cartB := cart // (received via replication)
cartB.Version.Increment("replicaB")

// Detect conflict
if cart.Version.Concurrent(cartB.Version) {
    // Merge carts and versions
    merged := mergeCartItems(cart.Items, cartB.Items)
    mergedVersion := cart.Version.Copy()
    mergedVersion.Merge(cartB.Version)
    mergedVersion.Increment("coordinator") // Resolution marker
}
```

### Multi-Master Replication

```go
type Document struct {
    Key     string
    Content string
    Version *vclock.VersionVector
}

// Write at master A
docA := &Document{
    Key:     "doc1",
    Content: "Version A",
    Version: vclock.NewVersionVector(),
}
docA.Version.Increment("masterA")

// Replicate and write at master B
docB := docA // (received via replication)
docB.Content = "Version B"
docB.Version.Increment("masterB")

// On read, detect conflict
if docA.Version.Concurrent(docB.Version) {
    // Application-specific resolution
    // Option 1: Last-write-wins (merge + timestamp)
    // Option 2: Keep both as siblings
    // Option 3: Semantic merge of content
}
```

## License

MIT License - See LICENSE file

## Contributing

Contributions welcome! Please ensure:
- All tests pass (`go test`)
- Code is formatted (`go fmt`)
- New features include tests and examples
- Design decisions are documented

## Package Reference

### clock Package

**Purpose**: Vector clocks for distributed event ordering and causality tracking.

**Key Types**:
- `Clock` - Vector clock data structure
- `NodeID` - Node identifier (string alias)
- `Comparison` - Causal relation enum (Equal, Before, After, Concurrent)

**Key Functions**:
```go
NewClock(nodes ...NodeID) *Clock
```

**Common Patterns**:
```go
// Event happens-before tracking
clock.Increment("nodeA")
clock.Merge(remoteClockSnapshot)

// Causality detection
if clock1.HappenedBefore(clock2) {
    // clock1 → clock2 (causal)
}
if clock1.Concurrent(clock2) {
    // clock1 ∥ clock2 (conflict)
}
```

**When to Use**:
- Distributed protocol implementation
- Event ordering in message queues
- Distributed debugging/tracing
- Happens-before relationship tracking

**Documentation**: See [clock/README.md](clock/README.md)

---

### version Package

**Purpose**: Version vectors for per-object version tracking in multi-master replication.

**Key Types**:
- `VersionVector` - Version vector data structure
- `ReplicaID` - Replica identifier (string alias)
- `Comparison` - Causal relation enum (same as clock)

**Key Functions**:
```go
NewVersionVector(replicas ...ReplicaID) *VersionVector
```

**Common Patterns**:
```go
// Object versioning
version.Increment("replicaA")

// Conflict detection
if version1.Concurrent(version2) {
    // Divergent versions - resolve conflict
}

// Merge on read
merged := version1.Copy()
merged.Merge(version2)
```

**When to Use**:
- Dynamo/Riak-style databases
- Shopping cart merging
- Collaborative editing
- Multi-master replication
- CRDT implementation

**Documentation**: See [version/README.md](version/README.md)

---

### message Package

**Purpose**: Versioned message framing with pluggable serialization and embedded vector clocks.

**Key Types**:
- `Message` - Message structure with clock, payload, metadata
- `Serializer` - Interface for pluggable serialization
- `JSONSerializer` - Human-readable JSON format
- `CBORSerializer` - Binary CBOR format (compact)
- `Registry` - Serializer registry for runtime selection

**Key Functions**:
```go
NewMessage(senderID string, clock *clock.Clock, payload []byte) (*Message, error)
NewCBORSerializer() (*CBORSerializer, error)
```

**Common Patterns**:
```go
// Create and send
msg, _ := message.NewMessage("node1", clock, data)
msg.WithMetadata("trace_id", "123")

ser := &message.JSONSerializer{}
wireData, _ := ser.Marshal(msg)

// Receive and process
received, _ := ser.Unmarshal(wireData)
localClock.Merge(received.Clock)
```

**When to Use**:
- Building distributed applications
- Need versioned message protocol
- Want pluggable serialization
- Handling untrusted input (validation)
- Automatic clock embedding in messages

**Security Features**:
- Size limits (16MB message, 15MB payload)
- Strict validation on all fields
- CBOR hardening (max map/array sizes)
- Control character rejection

**Documentation**: See [message/README.md](message/README.md)

---

### transport Package

**Purpose**: Transport abstraction for network communication (optional layer).

**Key Types**:
- `Transport` - Transport interface (Listen, Dial, Close)
- `Connection` - Bidirectional connection (Send, Recv, Close)
- `Listener` - Accept inbound connections
- `Message` - Transport-level message (opaque body)

**Implementations**:
- `MemoryTransport` - In-memory, deterministic (testing)
- `MockTransport` - Controllable mock (testing)
- `TCPTransport` - Production TCP transport

**Key Functions**:
```go
NewMemoryTransport(config TransportConfig) *MemoryTransport
NewMockTransport(config TransportConfig) *MockTransport
NewTCPTransport(config TransportConfig) *TCPTransport
```

**Common Patterns**:
```go
// Server
tr := transport.NewTCPTransport(config)
listener, _ := tr.Listen(ctx, "localhost:8080")
conn, _ := listener.Accept(ctx)
msg, _ := conn.Recv(ctx)

// Client
conn, _ := tr.Dial(ctx, "localhost:8080")
conn.Send(ctx, &transport.Message{...})
```

**When to Use**:
- Building complete distributed system
- Need transport abstraction
- Want deterministic testing
- Implementing connection pooling

**When to Skip**:
- Using existing RPC (gRPC, Thrift)
- Already have networking layer
- Only need causality tracking

**Documentation**: See [transport/README.md](transport/README.md)

---

## Package Dependencies

```
Application
      │
      ├─── clock (zero deps)
      │
      ├─── version (zero deps)
      │
      ├─── message
      │      ├─── clock
      │      └── github.com/fxamacker/cbor/v2
      │
      └─── transport (zero deps, optional)
             └── (your choice: TCP, QUIC, etc.)
```

**Dependency Philosophy**:
- Core packages (`clock`, `version`) have **zero dependencies**
- Message framing has **one external dependency** (CBOR)
- Transport is **optional** and dependency-free
- You choose what to include

## References

- Lamport, L. (1978). "Time, Clocks, and the Ordering of Events in a Distributed System"
- Fidge, C. (1988). "Timestamps in Message-Passing Systems That Preserve the Partial Ordering"
- Mattern, F. (1988). "Virtual Time and Global States of Distributed Systems"

## FAQ

**Q: When should I use vector clocks vs. timestamps?**  
A: Use vector clocks when:
- You need to detect causality violations
- Nodes don't have synchronized clocks  
- You handle concurrent writes

Use timestamps when:
- You only need total ordering
- Clocks are synchronized (NTP)
- You can tolerate small skew

**Q: How do I serialize clocks for network transmission?**  
A: Use a proper serialization format:
```go
// Option 1: JSON
data, _ := json.Marshal(clock)

// Option 2: Protocol Buffers (recommended for performance)
// Define .proto message with map<string, int64> field

// Option 3: Custom binary format
// See implementation of String() for inspiration
```

**Q: Can I use this with databases?**  
A: Yes. Store clocks as JSON, binary blobs, or decomposed into rows. Comparisons can detect write conflicts in optimistic locking schemes.

**Q: What's the maximum number of nodes?**  
A: Practically unlimited, but performance degrades linearly. Typical systems have 3-100 nodes. For 1000+ nodes, consider Dotted Version Vectors or other compressed variants.

**Q: How do I handle node failures?**  
A: Vector clocks themselves don't detect failures. Combine with failure detectors or heartbeats. Offline nodes simply stop incrementing their clock value.

**Q: Should I garbage collect old nodes?**  
A: Depends on your application. If nodes are ephemeral, you may want to prune nodes not seen for N operations. This requires application-level logic.
