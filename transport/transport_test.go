package transport

import (
	"context"
	"testing"
	"time"
)

// TestMemoryTransport tests the in-memory transport implementation.
func TestMemoryTransport_BasicSendRecv(t *testing.T) {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	// Create listener
	ctx := context.Background()
	listener, err := transport.Listen(ctx, "server:5000")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	// Accept connection in goroutine
	var serverConn Connection
	var acceptErr error
	go func() {
		var ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		serverConn, acceptErr = listener.Accept(ctx)
	}()

	// Dial connection
	clientCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	clientConn, err := transport.Dial(clientCtx, "server:5000")
	cancel()
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}

	// Wait for accept to complete
	time.Sleep(100 * time.Millisecond)
	if acceptErr != nil {
		t.Fatalf("Accept failed: %v", acceptErr)
	}
	if serverConn == nil {
		t.Fatal("serverConn is nil")
	}

	// Send message from client to server
	msg := &Message{
		From: "client",
		To:   "server",
		Body: []byte("hello world"),
	}

	if err := clientConn.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Receive message
	recvCtx, recvCancel := context.WithTimeout(context.Background(), 5*time.Second)
	received, err := serverConn.Recv(recvCtx)
	recvCancel()
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}

	if received.From != msg.From || received.To != msg.To {
		t.Errorf("Message mismatch: got %+v, want %+v", received, msg)
	}

	clientConn.Close()
	serverConn.Close()
	listener.Close()
}

// TestMemoryTransport_Concurrent tests concurrent sends and receives.
func TestMemoryTransport_Concurrent(t *testing.T) {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listener, _ := transport.Listen(context.Background(), "server:5001")

	// Accept in goroutine
	doneChan := make(chan Connection, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _ := listener.Accept(ctx)
		doneChan <- conn
	}()

	// Dial
	clientConn, _ := transport.Dial(context.Background(), "server:5001")
	serverConn := <-doneChan

	// Send multiple messages concurrently
	numMessages := 100
	for i := 0; i < numMessages; i++ {
		msg := &Message{
			From:  "client",
			To:    "server",
			Seq:   uint64(i),
			Body:  []byte("msg"),
		}
		clientConn.Send(context.Background(), msg)
	}

	// Receive all
	for i := 0; i < numMessages; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		msg, err := serverConn.Recv(ctx)
		cancel()
		if err != nil {
			t.Fatalf("Recv %d failed: %v", i, err)
		}
		if msg.Seq != uint64(i) {
			t.Errorf("Message %d has wrong seq: %d", i, msg.Seq)
		}
	}

	clientConn.Close()
	serverConn.Close()
	listener.Close()
}

// TestMemoryTransport_ClosedConnection tests errors on closed connections.
func TestMemoryTransport_ClosedConnection(t *testing.T) {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listener, _ := transport.Listen(context.Background(), "server:5002")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "server:5002")

	// Close connection
	clientConn.Close()

	// Further operations should fail
	msg := &Message{From: "client", Body: []byte("test")}
	if err := clientConn.Send(context.Background(), msg); err != ErrConnectionClosed {
		t.Errorf("Send on closed conn should fail with ErrConnectionClosed, got %v", err)
	}

	_, err := clientConn.Recv(context.Background())
	if err != ErrConnectionClosed {
		t.Errorf("Recv on closed conn should fail with ErrConnectionClosed, got %v", err)
	}

	listener.Close()
}

// TestMemoryTransport_ContextCancellation tests context cancellation.
func TestMemoryTransport_RecvContextCancellation(t *testing.T) {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listener, _ := transport.Listen(context.Background(), "server:5003")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "server:5003")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := clientConn.Recv(ctx)
	if err != ErrContextCancelled {
		t.Errorf("Recv with cancelled context should return ErrContextCancelled, got %v", err)
	}

	clientConn.Close()
	listener.Close()
}

// TestMemoryTransport_MultipleListeners tests multiple listeners.
func TestMemoryTransport_MultipleListeners(t *testing.T) {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listen1, _ := transport.Listen(context.Background(), "server:5004")
	listen2, _ := transport.Listen(context.Background(), "server:5005")

	// Accept from both
	done1 := make(chan Connection, 1)
	done2 := make(chan Connection, 1)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _ := listen1.Accept(ctx)
		done1 <- conn
	}()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, _ := listen2.Accept(ctx)
		done2 <- conn
	}()

	conn1, _ := transport.Dial(context.Background(), "server:5004")
	conn2, _ := transport.Dial(context.Background(), "server:5005")

	server1 := <-done1
	server2 := <-done2

	// Send to each
	msg1 := &Message{From: "client", Body: []byte("to server1")}
	msg2 := &Message{From: "client", Body: []byte("to server2")}

	conn1.Send(context.Background(), msg1)
	conn2.Send(context.Background(), msg2)

	// Receive from each
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	recv1, _ := server1.Recv(ctx)
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	recv2, _ := server2.Recv(ctx)
	cancel()

	if string(recv1.Body) != "to server1" {
		t.Errorf("Wrong message to server1: %s", string(recv1.Body))
	}
	if string(recv2.Body) != "to server2" {
		t.Errorf("Wrong message to server2: %s", string(recv2.Body))
	}

	conn1.Close()
	conn2.Close()
	server1.Close()
	server2.Close()
	listen1.Close()
	listen2.Close()
}

// TestMockTransport_BasicOperation tests mock transport setup.
func TestMockTransport_BasicOperation(t *testing.T) {
	transport := NewMockTransport(DefaultConfig())
	defer transport.Close()

	listener, err := transport.Listen(context.Background(), "server:6000")
	if err != nil {
		t.Fatalf("Listen failed: %v", err)
	}

	var serverConn Connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		serverConn, _ = listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "server:6000")
	time.Sleep(100 * time.Millisecond) // Let accept complete

	msg := &Message{From: "client", Body: []byte("test")}
	clientConn.Send(context.Background(), msg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	received, _ := serverConn.Recv(ctx)
	cancel()

	if string(received.Body) != "test" {
		t.Errorf("Wrong message body: %s", string(received.Body))
	}

	clientConn.Close()
	serverConn.Close()
	listener.Close()
}

// TestMockTransport_FailureInjection tests failure injection hooks.
func TestMockTransport_SendFailure(t *testing.T) {
	transport := NewMockTransport(DefaultConfig())
	defer transport.Close()

	// Set global send hook to inject failure
	transport.SetOnSend(func(m *Message) error {
		if m.Seq == 5 {
			return ErrDialFailed
		}
		return nil
	})

	listener, _ := transport.Listen(context.Background(), "server:6001")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "server:6001")

	// Send should succeed
	msg1 := &Message{From: "client", Seq: 1}
	if err := clientConn.Send(context.Background(), msg1); err != nil {
		t.Errorf("Send seq=1 failed: %v", err)
	}

	// Send should fail
	msg5 := &Message{From: "client", Seq: 5}
	if err := clientConn.Send(context.Background(), msg5); err != ErrDialFailed {
		t.Errorf("Send seq=5 should fail, got %v", err)
	}

	clientConn.Close()
	listener.Close()
}

// TestMockTransport_Hooks tests recv hooks.
func TestMockTransport_RecvHook(t *testing.T) {
	transport := NewMockTransport(DefaultConfig())
	defer transport.Close()

	hookCalled := false
	transport.SetOnRecv(func(m *Message) {
		hookCalled = true
	})

	listener, _ := transport.Listen(context.Background(), "server:6002")

	var serverConn Connection
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		serverConn, _ = listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "server:6002")
	time.Sleep(100 * time.Millisecond)

	// Send message from client
	msg := &Message{From: "client", Body: []byte("test")}
	clientConn.Send(context.Background(), msg)

	// Receive on server side triggers the hook
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if serverConn != nil {
		serverConn.Recv(ctx)
	}

	clientConn.Close()
	if serverConn != nil {
		serverConn.Close()
	}
	listener.Close()

	// Hook should have been called
	if !hookCalled {
		t.Error("Recv hook was not called")
	}
}

// BenchmarkMemoryTransport benchmarks in-memory transport.
func BenchmarkMemoryTransport_SendRecv(b *testing.B) {
	transport := NewMemoryTransport(DefaultConfig())
	defer transport.Close()

	listener, _ := transport.Listen(context.Background(), "server:7000")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		listener.Accept(ctx)
	}()

	clientConn, _ := transport.Dial(context.Background(), "server:7000")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		msg := &Message{From: "client", Seq: uint64(i)}
		clientConn.Send(context.Background(), msg)
	}

	clientConn.Close()
	listener.Close()
}
