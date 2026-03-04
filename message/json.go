package message

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// JSONSerializer implements Serializer using JSON encoding.
//
// JSON is human-readable and widely supported, but less efficient than binary formats.
type JSONSerializer struct{}

// Marshal serializes a message to JSON.
func (s *JSONSerializer) Marshal(msg *Message) ([]byte, error) {
	if msg == nil {
		return nil, fmt.Errorf("cannot marshal nil message")
	}

	// Validate before marshaling
	if err := msg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Use a buffer for efficient encoding
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "") // Compact JSON

	if err := encoder.Encode(msg); err != nil {
		return nil, fmt.Errorf("json encoding failed: %w", err)
	}

	data := buf.Bytes()

	// Check size limit
	if len(data) > MaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	return data, nil
}

// Unmarshal deserializes JSON to a message.
func (s *JSONSerializer) Unmarshal(data []byte) (*Message, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty data", ErrInvalidEncoding)
	}

	if len(data) > MaxMessageSize {
		return nil, ErrMessageTooLarge
	}

	var msg Message
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields() // Strict parsing

	if err := decoder.Decode(&msg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidEncoding, err)
	}

	// Validate the deserialized message
	if err := msg.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &msg, nil
}

// Name returns the serializer name.
func (s *JSONSerializer) Name() string {
	return "json"
}
