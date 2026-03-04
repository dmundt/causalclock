package message_test

import (
	"fmt"
	"log"

	"github.com/dmundt/causalclock/clock"
	"github.com/dmundt/causalclock/message"
)

// Example_basicMessage demonstrates creating and serializing a basic message.
func Example_basicMessage() {
	// Create a vector clock
	clk := clock.NewClock()
	clk.Increment("node1")

	// Create a message
	msg, err := message.NewMessage("node1", clk, []byte("Hello, distributed world!"))
	if err != nil {
		log.Fatal(err)
	}

	// Serialize with JSON
	jsonSer := &message.JSONSerializer{}
	data, err := jsonSer.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Message serialized (%d bytes)\n", len(data))

	// Deserialize
	decoded, err := jsonSer.Unmarshal(data)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Sender: %s\n", decoded.SenderID)
	fmt.Printf("Payload: %s\n", string(decoded.Payload))

	// Output:
	// Message serialized (126 bytes)
	// Sender: node1
	// Payload: Hello, distributed world!
}

// Example_jsonVsCbor demonstrates comparing JSON and CBOR serialization.
func Example_jsonVsCbor() {
	// Create a message with a moderately complex clock
	clk := clock.NewClock()
	clk.Increment("node1")
	clk.Increment("node2")
	clk.Increment("node3")

	msg, _ := message.NewMessage("node1", clk, []byte("test payload"))
	msg.WithMetadata("priority", "high").WithMetadata("type", "sync")

	// JSON serialization
	jsonSer := &message.JSONSerializer{}
	jsonData, _ := jsonSer.Marshal(msg)

	// CBOR serialization
	cborSer, err := message.NewCBORSerializer()
	if err != nil {
		log.Fatal(err)
	}
	cborData, _ := cborSer.Marshal(msg)

	fmt.Printf("JSON size: %d bytes\n", len(jsonData))
	fmt.Printf("CBOR size: %d bytes\n", len(cborData))
	fmt.Printf("CBOR is %.1f%% of JSON size\n", 100.0*float64(len(cborData))/float64(len(jsonData)))

	// Both should round-trip correctly
	jsonDecoded, _ := jsonSer.Unmarshal(jsonData)
	cborDecoded, _ := cborSer.Unmarshal(cborData)

	fmt.Printf("JSON round-trip: %s\n", jsonDecoded.SenderID)
	fmt.Printf("CBOR round-trip: %s\n", cborDecoded.SenderID)

	// Output:
	// JSON size: 171 bytes
	// CBOR size: 85 bytes
	// CBOR is 49.7% of JSON size
	// JSON round-trip: node1
	// CBOR round-trip: node1
}

// Example_untrustedInput demonstrates safely handling untrusted input.
func Example_untrustedInput() {
	// Simulate receiving data from an untrusted source
	malformedInputs := [][]byte{
		[]byte(`{"version":0}`),                          // Invalid version
		[]byte(`{"version":1,"sender_id":"","clock":{}}`), // Empty sender ID
		[]byte(`{invalid json`),                          // Malformed JSON
		make([]byte, message.MaxMessageSize+1),           // Too large
	}

	jsonSer := &message.JSONSerializer{}

	for i, data := range malformedInputs {
		msg, err := jsonSer.Unmarshal(data)
		if err != nil {
			fmt.Printf("Input %d: safely rejected\n", i+1)
		} else {
			// This shouldn't happen with our validation
			fmt.Printf("Input %d: accepted (sender: %s)\n", i+1, msg.SenderID)
		}
	}

	// Output:
	// Input 1: safely rejected
	// Input 2: safely rejected
	// Input 3: safely rejected
	// Input 4: safely rejected
}

// Example_vectorClockIntegration demonstrates using messages with real vector clocks.
func Example_vectorClockIntegration() {
	// Simulate a simple distributed system with 3 nodes
	nodes := []string{"node1", "node2", "node3"}

	// Each node maintains its own clock
	clocks := make(map[string]*clock.Clock)
	for _, node := range nodes {
		clocks[node] = clock.NewClock()
	}

	// Node1 performs an action
	clocks["node1"].Increment("node1")

	// Node1 sends a message to node2
	msg1, _ := message.NewMessage("node1", clocks["node1"], []byte("event from node1"))

	// Serialize and send (simulated)
	ser := &message.JSONSerializer{}
	data, _ := ser.Marshal(msg1)

	// Node2 receives and deserializes
	received, _ := ser.Unmarshal(data)

	// Node2 merges the clock (happens-before relationship)
	clocks["node2"].Merge(received.Clock)
	clocks["node2"].Increment("node2")

	fmt.Printf("Node2 received from: %s\n", received.SenderID)
	fmt.Printf("Node2 payload: %s\n", string(received.Payload))
	fmt.Printf("Node2 clock updated: %s\n", clocks["node2"].String())

	// Output:
	// Node2 received from: node1
	// Node2 payload: event from node1
	// Node2 clock updated: {node1:1, node2:1}
}

// Example_registry demonstrates using the serializer registry.
func Example_registry() {
	// Use the default registry
	reg := message.DefaultRegistry

	// Choose serializer at runtime
	serializerName := "json" // Could be from config, command-line, etc.

	ser, ok := reg.Get(serializerName)
	if !ok {
		log.Fatalf("Serializer %q not found", serializerName)
	}

	clk := clock.NewClock()
	clk.Increment("node1")

	msg, _ := message.NewMessage("node1", clk, []byte("flexible serialization"))
	data, _ := ser.Marshal(msg)

	fmt.Printf("Using %s serializer\n", ser.Name())
	fmt.Printf("Serialized %d bytes\n", len(data))

	// Output:
	// Using json serializer
	// Serialized 122 bytes
}

// Example_metadata demonstrates adding metadata to messages.
func Example_metadata() {
	clk := clock.NewClock()
	clk.Increment("node1")

	msg, _ := message.NewMessage("node1", clk, []byte("data"))

	// Add metadata for routing, tracing, etc.
	msg.WithMetadata("trace_id", "abc123").
		WithMetadata("priority", "high").
		WithMetadata("retry_count", "0")

	fmt.Printf("Sender: %s\n", msg.SenderID)
	fmt.Printf("Trace ID: %s\n", msg.Metadata["trace_id"])
	fmt.Printf("Priority: %s\n", msg.Metadata["priority"])

	// Output:
	// Sender: node1
	// Trace ID: abc123
	// Priority: high
}
