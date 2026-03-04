package message

import (
	"bytes"
	"fmt"
	"time"

	"github.com/dmundt/causalclock/clock"
	"github.com/fxamacker/cbor/v2"
)

// CBORSerializer implements Serializer using CBOR (Concise Binary Object Representation).
//
// CBOR is more compact than JSON and designed for constrained environments.
// Spec: RFC 8949
type CBORSerializer struct {
	encMode cbor.EncMode
	decMode cbor.DecMode
}

// NewCBORSerializer creates a new CBOR serializer with safe defaults.
func NewCBORSerializer() (*CBORSerializer, error) {
	// Configure encoder with secure defaults
	encMode, err := cbor.EncOptions{
		Sort: cbor.SortBytewiseLexical, // Deterministic encoding
		Time: cbor.TimeRFC3339,         // Standard time format
	}.EncMode()
	if err != nil {
		return nil, fmt.Errorf("cbor encoder config failed: %w", err)
	}

	// Configure decoder with strict validation
	decMode, err := cbor.DecOptions{
		MaxMapPairs:      1000,                      // Limit map size
		MaxArrayElements: 10000,                     // Limit array size
		MaxNestedLevels:  16,                        // Prevent deep nesting attacks
		DupMapKey:        cbor.DupMapKeyEnforcedAPF, // Reject duplicate keys
	}.DecMode()
	if err != nil {
		return nil, fmt.Errorf("cbor decoder config failed: %w", err)
	}

	return &CBORSerializer{
		encMode: encMode,
		decMode: decMode,
	}, nil
}

// cborMessage is a helper struct for CBOR serialization with explicit clock data.
type cborMessage struct {
	Version   uint8             `cbor:"v"`
	SenderID  string            `cbor:"s"`
	ClockData map[string]int64  `cbor:"c"`
	Payload   []byte            `cbor:"p"`
	Timestamp int64             `cbor:"t,omitempty"`
	Metadata  map[string]string `cbor:"m,omitempty"`
}

// Marshal serializes a message to CBOR.
func (s *CBORSerializer) Marshal(msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("cannot marshal nil message")
	}

	// Validate before marshaling
	if err := msg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Convert clock to map
	clockData := make(map[string]int64)
	if msg.Clock != nil {
		for _, node := range msg.Clock.Nodes() {
			clockData[string(node)] = msg.Clock.Get(node)
		}
	}

	helper := cborMessage{
		Version:   msg.Version,
		SenderID:  msg.SenderID,
		ClockData: clockData,
		Payload:   msg.Payload,
		Metadata:  msg.Metadata,
	}

	if !msg.Timestamp.IsZero() {
		helper.Timestamp = msg.Timestamp.Unix()
	}

	var buf bytes.Buffer
	encoder := s.encMode.NewEncoder(&buf)

	if err := encoder.Encode(helper); err != nil {
		return nil, fmt.Errorf("cbor encoding failed: %w", err)
	}

	data := buf.Bytes()

	// Check size limit
	if len(data) > MaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	return data, nil
}

// Unmarshal deserializes CBOR to a message.
func (s *CBORSerializer) Unmarshal(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty data", ErrInvalidEncoding)
	}

	if len(data) > MaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	var helper cborMessage
	decoder := s.decMode.NewDecoder(bytes.NewReader(data))

	if err := decoder.Decode(&helper); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEncoding, err)
	}

	// Reconstruct clock from map
	clk := clock.NewClock()
	for node, value := range helper.ClockData {
		if value < 0 {
			return nil, fmt.Errorf("invalid clock value for node %s: %d", node, value)
		}

		// Set the value by incrementing a temporary clock
		tempClock := clock.NewClock()
		for i := int64(0); i < value; i++ {
			tempClock.Increment(clock.NodeID(node))
		}
		clk.Merge(tempClock)
	}

	msg := &Message{
		Version:  helper.Version,
		SenderID: helper.SenderID,
		Clock:    clk,
		Payload:  helper.Payload,
		Metadata: helper.Metadata,
	}

	if helper.Timestamp > 0 {
		msg.Timestamp = time.Unix(helper.Timestamp, 0)
	}

	// Validate the deserialized message
	if err := msg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return msg, nil
}

// Name returns the serializer name.
func (s *CBORSerializer) Name() string {
	return "cbor"
}
