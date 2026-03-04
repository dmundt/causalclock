package message

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dmundt/causalclock/clock"
)

// TestNewMessage tests message creation.
func TestNewMessage(t *testing.T) {
	clk := clock.NewClock()
	clk.Increment("node1")

	tests := []struct {
		name     string
		senderID string
		clock    *clock.Clock
		payload  []byte
		wantErr  bool
		errType  error
	}{
		{
			name:     "valid message",
			senderID: "node1",
			clock:    clk,
			payload:  []byte("test payload"),
			wantErr:  false,
		},
		{
			name:     "empty sender ID",
			senderID: "",
			clock:    clk,
			payload:  []byte("test"),
			wantErr:  true,
			errType:  ErrInvalidSenderID,
		},
		{
			name:     "nil clock",
			senderID: "node1",
			clock:    nil,
			payload:  []byte("test"),
			wantErr:  true,
			errType:  ErrNilClock,
		},
		{
			name:     "payload too large",
			senderID: "node1",
			clock:    clk,
			payload:  make([]byte, MaxPayloadSize+1),
			wantErr:  true,
			errType:  ErrPayloadTooLarge,
		},
		{
			name:     "sender ID too long",
			senderID: strings.Repeat("a", MaxSenderIDLength+1),
			clock:    clk,
			payload:  []byte("test"),
			wantErr:  true,
			errType:  ErrInvalidSenderID,
		},
		{
			name:     "sender ID with control characters",
			senderID: "node\x00test",
			clock:    clk,
			payload:  []byte("test"),
			wantErr:  true,
			errType:  ErrInvalidSenderID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := NewMessage(tt.senderID, tt.clock, tt.payload)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errType != nil && !errors.Is(err, tt.errType) {
				t.Errorf("NewMessage() error = %v, want error type %v", err, tt.errType)
			}

			if !tt.wantErr {
				if msg == nil {
					t.Error("NewMessage() returned nil message")
					return
				}

				if msg.Version != CurrentVersion {
					t.Errorf("NewMessage() version = %d, want %d", msg.Version, CurrentVersion)
				}

				if msg.SenderID != tt.senderID {
					t.Errorf("NewMessage() senderID = %q, want %q", msg.SenderID, tt.senderID)
				}

				if !bytes.Equal(msg.Payload, tt.payload) {
					t.Errorf("NewMessage() payload mismatch")
				}

				// Check clock was copied
				if msg.Clock == tt.clock {
					t.Error("NewMessage() did not copy clock (same pointer)")
				}

				// Check timestamp is recent
				if time.Since(msg.Timestamp) > time.Second {
					t.Error("NewMessage() timestamp not recent")
				}
			}
		})
	}
}

// TestMessageValidate tests message validation.
func TestMessageValidate(t *testing.T) {
	validClock := clock.NewClock()
	validClock.Increment("node1")

	tests := []struct {
		name    string
		msg     *Message
		wantErr bool
		errType error
	}{
		{
			name: "valid message",
			msg: &Message{
				Version:  CurrentVersion,
				SenderID: "node1",
				Clock:    validClock,
				Payload:  []byte("test"),
			},
			wantErr: false,
		},
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name: "invalid version (0)",
			msg: &Message{
				Version:  0,
				SenderID: "node1",
				Clock:    validClock,
				Payload:  []byte("test"),
			},
			wantErr: true,
			errType: ErrInvalidVersion,
		},
		{
			name: "invalid version (too high)",
			msg: &Message{
				Version:  255,
				SenderID: "node1",
				Clock:    validClock,
				Payload:  []byte("test"),
			},
			wantErr: true,
			errType: ErrInvalidVersion,
		},
		{
			name: "nil clock",
			msg: &Message{
				Version:  CurrentVersion,
				SenderID: "node1",
				Clock:    nil,
				Payload:  []byte("test"),
			},
			wantErr: true,
			errType: ErrNilClock,
		},
		{
			name: "payload too large",
			msg: &Message{
				Version:  CurrentVersion,
				SenderID: "node1",
				Clock:    validClock,
				Payload:  make([]byte, MaxPayloadSize+1),
			},
			wantErr: true,
			errType: ErrPayloadTooLarge,
		},
		{
			name: "metadata key too large",
			msg: &Message{
				Version:  CurrentVersion,
				SenderID: "node1",
				Clock:    validClock,
				Payload:  []byte("test"),
				Metadata: map[string]string{
					strings.Repeat("a", 300): "value",
				},
			},
			wantErr: true,
		},
		{
			name: "metadata value too large",
			msg: &Message{
				Version:  CurrentVersion,
				SenderID: "node1",
				Clock:    validClock,
				Payload:  []byte("test"),
				Metadata: map[string]string{
					"key": strings.Repeat("a", 2000),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.msg.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && tt.errType != nil && !errors.Is(err, tt.errType) {
				t.Errorf("Validate() error = %v, want error type %v", err, tt.errType)
			}
		})
	}
}

// TestJSONSerializer tests JSON serialization.
func TestJSONSerializer(t *testing.T) {
	ser := &JSONSerializer{}

	clk := clock.NewClock()
	clk.Increment("node1")
	clk.Increment("node2")

	msg, err := NewMessage("sender1", clk, []byte("test payload"))
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}
	msg.WithMetadata("key1", "value1")

	// Test Marshal
	data, err := ser.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}

	// Test Unmarshal
	msg2, err := ser.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify fields
	if msg2.Version != msg.Version {
		t.Errorf("Version mismatch: got %d, want %d", msg2.Version, msg.Version)
	}

	if msg2.SenderID != msg.SenderID {
		t.Errorf("SenderID mismatch: got %q, want %q", msg2.SenderID, msg.SenderID)
	}

	if !bytes.Equal(msg2.Payload, msg.Payload) {
		t.Error("Payload mismatch")
	}

	if msg2.Clock.Compare(msg.Clock) != clock.EqualCmp {
		t.Error("Clock mismatch")
	}

	if msg2.Metadata["key1"] != "value1" {
		t.Error("Metadata mismatch")
	}
}

// TestJSONSerializer_MalformedInput tests JSON deserialization with malformed data.
func TestJSONSerializer_MalformedInput(t *testing.T) {
	ser := &JSONSerializer{}

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			data:    []byte("{invalid json"),
			wantErr: true,
		},
		{
			name:    "data too large",
			data:    make([]byte, MaxMessageSize+1),
			wantErr: true,
		},
		{
			name:    "missing required fields",
			data:    []byte(`{"version":1}`),
			wantErr: true,
		},
		{
			name:    "invalid version",
			data:    []byte(`{"version":0,"sender_id":"test","clock":{},"payload":""}`),
			wantErr: true,
		},
		{
			name:    "null clock",
			data:    []byte(`{"version":1,"sender_id":"test","clock":null,"payload":"dGVzdA=="}`),
			wantErr: true,
		},
		{
			name:    "unknown fields (strict mode)",
			data:    []byte(`{"version":1,"sender_id":"test","clock":{},"payload":"","unknown_field":"value"}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ser.Unmarshal(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestCBORSerializer tests CBOR serialization.
func TestCBORSerializer(t *testing.T) {
	ser, err := NewCBORSerializer()
	if err != nil {
		t.Fatalf("NewCBORSerializer failed: %v", err)
	}

	clk := clock.NewClock()
	clk.Increment("node1")
	clk.Increment("node2")

	msg, err := NewMessage("sender1", clk, []byte("test payload"))
	if err != nil {
		t.Fatalf("NewMessage failed: %v", err)
	}

	// Test Marshal
	data, err := ser.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}

	// CBOR should be more compact than JSON
	jsonSer := &JSONSerializer{}
	jsonData, _ := jsonSer.Marshal(msg)
	if len(data) >= len(jsonData) {
		t.Logf("CBOR size %d >= JSON size %d (expected smaller)", len(data), len(jsonData))
	}

	// Test Unmarshal
	msg2, err := ser.Unmarshal(data)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify fields
	if msg2.Version != msg.Version {
		t.Errorf("Version mismatch")
	}

	if msg2.SenderID != msg.SenderID {
		t.Errorf("SenderID mismatch")
	}

	if !bytes.Equal(msg2.Payload, msg.Payload) {
		t.Error("Payload mismatch")
	}
}

// TestCBORSerializer_MalformedInput tests CBOR deserialization with malformed data.
func TestCBORSerializer_MalformedInput(t *testing.T) {
	ser, err := NewCBORSerializer()
	if err != nil {
		t.Fatalf("NewCBORSerializer failed: %v", err)
	}

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty data",
			data:    []byte{},
			wantErr: true,
		},
		{
			name:    "invalid CBOR",
			data:    []byte{0xFF, 0xFF, 0xFF},
			wantErr: true,
		},
		{
			name:    "data too large",
			data:    make([]byte, MaxMessageSize+1),
			wantErr: true,
		},
		{
			name:    "truncated data",
			data:    []byte{0xA1}, // Map with 1 element but no data
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ser.Unmarshal(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestMessageSize tests size estimation.
func TestMessageSize(t *testing.T) {
	clk := clock.NewClock()
	clk.Increment("node1")
	clk.Increment("node2")
	clk.Increment("node3")

	msg, _ := NewMessage("sender1", clk, []byte("test payload"))
	msg.WithMetadata("key1", "value1").WithMetadata("key2", "value2")

	size := msg.Size()
	if size <= 0 {
		t.Error("Size() returned non-positive value")
	}

	// Size should be reasonable
	if size > 1000 {
		t.Errorf("Size() = %d, seems too large for this message", size)
	}
}

// TestMessageWithMetadata tests metadata chaining.
func TestMessageWithMetadata(t *testing.T) {
	clk := clock.NewClock()
	msg, _ := NewMessage("sender1", clk, []byte("test"))

	msg.WithMetadata("k1", "v1").WithMetadata("k2", "v2").WithMetadata("k3", "v3")

	if len(msg.Metadata) != 3 {
		t.Errorf("Metadata length = %d, want 3", len(msg.Metadata))
	}

	if msg.Metadata["k1"] != "v1" || msg.Metadata["k2"] != "v2" || msg.Metadata["k3"] != "v3" {
		t.Error("Metadata values incorrect")
	}
}

// TestRegistry tests serializer registry.
func TestRegistry(t *testing.T) {
	reg := NewRegistry()

	jsonSer := &JSONSerializer{}
	reg.Register(jsonSer)

	// Test Get
	s, ok := reg.Get("json")
	if !ok {
		t.Error("Get() failed to find registered serializer")
	}

	if s.Name() != "json" {
		t.Errorf("Get() returned wrong serializer: %q", s.Name())
	}

	// Test missing serializer
	_, ok = reg.Get("missing")
	if ok {
		t.Error("Get() found non-existent serializer")
	}
}

// TestDefaultRegistry tests the default registry.
func TestDefaultRegistry(t *testing.T) {
	// JSON should be registered
	_, ok := DefaultRegistry.Get("json")
	if !ok {
		t.Error("DefaultRegistry missing JSON serializer")
	}

	// CBOR should be registered
	_, ok = DefaultRegistry.Get("cbor")
	if !ok {
		t.Error("DefaultRegistry missing CBOR serializer")
	}
}

// TestRoundTrip tests round-trip serialization with both formats.
func TestRoundTrip(t *testing.T) {
	clk := clock.NewClock()
	clk.Increment("node1")
	clk.Increment("node2")

	original, _ := NewMessage("sender1", clk, []byte("test payload"))
	original.WithMetadata("test", "value")

	serializers := []Serializer{
		&JSONSerializer{},
	}

	cborSer, err := NewCBORSerializer()
	if err == nil {
		serializers = append(serializers, cborSer)
	}

	for _, ser := range serializers {
		t.Run(ser.Name(), func(t *testing.T) {
			// Marshal
			data, err := ser.Marshal(original)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			// Unmarshal
			decoded, err := ser.Unmarshal(data)
			if err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Compare
			if decoded.Version != original.Version {
				t.Error("Version mismatch")
			}

			if decoded.SenderID != original.SenderID {
				t.Error("SenderID mismatch")
			}

			if !bytes.Equal(decoded.Payload, original.Payload) {
				t.Error("Payload mismatch")
			}

			if decoded.Clock.Compare(original.Clock) != clock.EqualCmp {
				t.Error("Clock mismatch")
			}
		})
	}
}

// TestNilMessage tests marshaling nil message.
func TestNilMessage(t *testing.T) {
	ser := &JSONSerializer{}

	_, err := ser.Marshal(nil)
	if err == nil {
		t.Error("Marshal(nil) should return error")
	}
}
