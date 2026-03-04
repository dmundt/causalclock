package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Example_basicUsage demonstrates basic message exchange with in-memory transport.
func Example_basicUsage() {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	// Create listener
	listener, _ := transport.Listen(context.Background(), "127.0.0.1:5000")

	// Accept connection in background
	var serverConn Connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		serverConn, _ = listener.Accept(ctx)
	}()

	// Client dials
	clientConn, _ := transport.Dial(context.Background(), "127.0.0.1:5000")

	// Wait for connection to establish
	time.Sleep(10 * time.Millisecond)

	// Send message
	msg := &Message{
		From: "Alice",
		To:   "Bob",
		Body: []byte("Hello Bob"),
	}

	clientConn.Send(context.Background(), msg)

	// Receive message
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	received, _ := serverConn.Recv(ctx)
	cancel()

	fmt.Printf("From: %s, To: %s, Body: %s\n", received.From, received.To, string(received.Body))
	// Output: From: Alice, To: Bob, Body: Hello Bob
}

// Example_vectorClockIntegration shows how to include vector clock data in messages.
// This pattern separates transport concerns from serialization.
func Example_vectorClockIntegration() {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listener, _ := transport.Listen(context.Background(), "node-a:5001")

	var serverConn Connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		serverConn, _ = listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "node-a:5001")
	time.Sleep(10 * time.Millisecond)

	// Simulate vector clock data (in real code, marshal actual Clock struct)
	vectorClockData, _ := json.Marshal(map[string]int64{
		"node-1": 3,
		"node-2": 1,
	})

	// Create message with vector clock
	msg := &Message{
		From:            "node-1",
		To:              "node-2",
		Body:            []byte("important event"),
		VectorClockData: vectorClockData,
		Timestamp:       time.Now(),
	}

	clientConn.Send(context.Background(), msg)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	received, _ := serverConn.Recv(ctx)
	cancel()

	// Deserialize vector clock
	var vc map[string]int64
	json.Unmarshal(received.VectorClockData, &vc)

	fmt.Printf("Event from %s with clock: %v\n", received.From, vc)
	// Output: Event from node-1 with clock: map[node-1:3 node-2:1]
}

// Example_concurrentExchange shows handling of concurrent message exchanges.
func Example_concurrentExchange() {
	// Simulate a 3-node system with in-memory transport
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	nodes := []string{"node-1:6000", "node-2:6001", "node-3:6002"}
	conns := make(map[string]Connection)

	// Set up listeners
	for _, node := range nodes {
		listener, _ := transport.Listen(context.Background(), node)

		go func(n string, l Listener) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, _ := l.Accept(ctx)
			if conn != nil {
				conns[n] = conn
			}
		}(node, listener)
	}

	// Wait for setup
	time.Sleep(50 * time.Millisecond)

	// Create client connections
	dialConns := make(map[string]Connection)
	for _, node := range nodes {
		client, _ := transport.Dial(context.Background(), node)
		dialConns[node] = client
	}

	// Wait for connections to establish
	time.Sleep(50 * time.Millisecond)

	// Broadcast message from node 1
	for i := 1; i < len(nodes); i++ {
		msg := &Message{
			From: "node-1:6000",
			To:   nodes[i],
			Body: []byte("broadcast message"),
			Seq:  uint64(i),
		}
		dialConns[nodes[i]].Send(context.Background(), msg)
	}

	// Receive broadcasted messages
	for i := 1; i < len(nodes); i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		msg, _ := conns[nodes[i]].Recv(ctx)
		cancel()
		if msg != nil {
			fmt.Printf("Node %d received: %s\n", i+1, string(msg.Body))
		}
	}

	// Cleanup
	for _, conn := range dialConns {
		conn.Close()
	}
}

// Example_mockTransportFailure demonstrates mock transport for testing failure scenarios.
func Example_mockTransportFailure() {
	// Create mock transport
	mock := NewMockTransport(DefaultConfig())
	defer mock.Close()

	listener, _ := mock.Listen(context.Background(), "server:7000")

	var serverConn Connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		serverConn, _ = listener.Accept(ctx)
	}()

	clientConn, _ := mock.Dial(context.Background(), "server:7000")
	time.Sleep(10 * time.Millisecond)

	// Inject fault: drop messages with Seq > 2
	mock.SetMessageDropper(func(m *Message) bool {
		return m.Seq > 2
	})

	// Send messages; some will be dropped
	for i := 1; i <= 4; i++ {
		msg := &Message{From: "client", Seq: uint64(i), Body: []byte(fmt.Sprintf("msg-%d", i))}
		clientConn.Send(context.Background(), msg)
	}

	// Try to receive
	successCount := 0
	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		msg, err := serverConn.Recv(ctx)
		cancel()

		if err != nil {
			break
		}
		if msg != nil {
			successCount++
		}
	}

	fmt.Printf("Successfully received %d messages out of 4\n", successCount)
	// Output: Successfully received 2 messages out of 4
}

// Example_connectionLifecycle shows connection lifecycle management.
func Example_connectionLifecycle() {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listener, _ := transport.Listen(context.Background(), "127.0.0.1:8000")

	var serverConn Connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		serverConn, _ = listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "127.0.0.1:8000")
	time.Sleep(10 * time.Millisecond)

	// Send a message
	msg := &Message{From: "A", Body: []byte("data")}
	clientConn.Send(context.Background(), msg)

	// Receive on server
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	_, _ = serverConn.Recv(ctx)
	cancel()

	// Close client
	clientConn.Close()
	fmt.Println("Client connection closed")

	// Further sends will fail
	if err := clientConn.Send(context.Background(), msg); err != nil {
		fmt.Println("Send on closed connection failed as expected")
	}

	// Clean up server
	serverConn.Close()
	listener.Close()

	// Output:
	// Client connection closed
	// Send on closed connection failed as expected
}

// Example_messageRouting demonstrates message routing with vector clocks.
// This shows how to build a higher-level construct on top of transport.
func Example_messageRouting() {
	// Create a simple message router that includes vector clock tracking
	type VectorClockMessage struct {
		From    string
		To      string
		Clock   map[string]int64
		Payload string
	}

	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	// Set up a coordinator node
	listener, _ := transport.Listen(context.Background(), "coordinator:9000")

	var coordConn Connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		coordConn, _ = listener.Accept(ctx)
	}()

	// Client sends a message through coordinator
	clientConn, _ := transport.Dial(context.Background(), "coordinator:9000")
	time.Sleep(10 * time.Millisecond)

	// Create a message with embedded clock
	vcMsg := VectorClockMessage{
		From: "source",
		To:   "dest",
		Clock: map[string]int64{
			"source": 1,
			"dest":   0,
		},
		Payload: "data",
	}

	// Serialize and send
	data, _ := json.Marshal(vcMsg)
	transportMsg := &Message{
		From: vcMsg.From,
		To:   vcMsg.To,
		Body: data,
	}

	clientConn.Send(context.Background(), transportMsg)

	// Coordinator receives
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	received, _ := coordConn.Recv(ctx)
	cancel()

	// Deserialize
	var deserialized VectorClockMessage
	json.Unmarshal(received.Body, &deserialized)

	fmt.Printf("Coordinator received: from=%s, clock=%v\n", deserialized.From, deserialized.Clock)
	// Output: Coordinator received: from=source, clock=map[dest:0 source:1]
}

// Example_requestResponse shows how to implement request-response pattern with vector clocks.
func Example_requestResponse() {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listener, _ := transport.Listen(context.Background(), "server:10000")

	// Server handler
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _ := listener.Accept(ctx)

		// Server receives request
		recvCtx, recvCancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, _ := conn.Recv(recvCtx)
		recvCancel()

		// Server sends response
		resp := &Message{
			From: "server",
			To:   req.From,
			Body: []byte("response"),
			Seq:  req.Seq,
		}
		conn.Send(context.Background(), resp)
	}()

	// Client sends request and waits for response
	clientConn, _ := transport.Dial(context.Background(), "server:10000")
	time.Sleep(10 * time.Millisecond)

	req := &Message{
		From: "client",
		Body: []byte("request"),
		Seq:  42,
	}
	clientConn.Send(context.Background(), req)

	// Wait for response
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	resp, _ := clientConn.Recv(ctx)
	cancel()

	if resp != nil && resp.Seq == 42 {
		fmt.Printf("Request-response completed: %s\n", string(resp.Body))
	}

	// Output: Request-response completed: response
}

// Example_peerToPeer shows implementing a peer-to-peer message exchange pattern.
func Example_peerToPeer() {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	// Set up two peers with bidirectional communication
	peer1Addr := "peer-1:11000"
	peer2Addr := "peer-2:11001"

	conns := make(map[string]Connection)
	var mu sync.Mutex

	// Both listen
	for addr := range map[string]bool{peer1Addr: true, peer2Addr: true} {
		listener, _ := transport.Listen(context.Background(), addr)
		go func(a string, l Listener) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, _ := l.Accept(ctx)
			mu.Lock()
			conns[a] = conn
			mu.Unlock()
		}(addr, listener)
	}

	time.Sleep(50 * time.Millisecond)

	// Exchange
	peer1Conn, _ := transport.Dial(context.Background(), peer2Addr)
	_, _ = transport.Dial(context.Background(), peer1Addr) // peer2 also dials for bidirectional

	time.Sleep(50 * time.Millisecond)

	// Peer 1 sends to peer 2
	msg := &Message{From: "peer-1", Body: []byte("hello from peer 1")}
	peer1Conn.Send(context.Background(), msg)

	mu.Lock()
	recv, ok := conns[peer2Addr]
	mu.Unlock()

	if ok && recv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		received, _ := recv.Recv(ctx)
		cancel()
		fmt.Printf("Peer 2 received: %s\n", string(received.Body))
	}

	// Output: Peer 2 received: hello from peer 1
}
