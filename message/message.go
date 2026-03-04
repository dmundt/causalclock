// Package message provides a versioned message-framing layer for distributed vector-clock systems.
//
// Key features:
//   - Pluggable serialization (JSON, CBOR, Protobuf-ready)
//   - Versioned message format for backward compatibility
//   - Safe validation for untrusted input
//   - Size limits to prevent resource exhaustion
//   - Clock data included in every message
package message

import (
	"errors"
	"fmt"
	"time"

	"github.com/dmundt/causalclock/clock"
)

const (
	// CurrentVersion is the current message format version.
	CurrentVersion = 1

	// MaxMessageSize is the maximum allowed message size (16MB).
	MaxMessageSize = 16 * 1024 * 1024

	// MaxSenderIDLength is the maximum sender ID length.
	MaxSenderIDLength = 256

	// MaxPayloadSize is the maximum payload size (15MB).
	MaxPayloadSize = 15 * 1024 * 1024
)

var (
	// ErrInvalidVersion indicates an unsupported message version.
	ErrInvalidVersion = errors.New("invalid message version")

	// ErrMessageTooLarge indicates the message exceeds size limits.
	ErrMessageTooLarge = errors.New("message too large")

	// ErrInvalidSenderID indicates the sender ID is invalid.
	ErrInvalidSenderID = errors.New("invalid sender ID")

	// ErrNilClock indicates the vector clock is nil.
	ErrNilClock = errors.New("nil vector clock")

	// ErrPayloadTooLarge indicates the payload exceeds size limits.
	ErrPayloadTooLarge = errors.New("payload too large")

	// ErrInvalidEncoding indicates the message encoding is invalid.
	ErrInvalidEncoding = errors.New("invalid message encoding")
)

// Message represents a versioned distributed message with vector clock metadata.
//
// Messages are designed to be safe for untrusted input with strict validation.
type Message struct {
	// Version is the message format version (for backward compatibility).
	Version uint8 `json:"version" cbor:"v"`

	// SenderID uniquely identifies the sender node.
	SenderID string `json:"sender_id" cbor:"s"`

	// Clock is the sender's vector clock at the time of sending.
	Clock *clock.Clock `json:"clock" cbor:"c"`

	// Payload is the application-specific message data.
	Payload []byte `json:"payload" cbor:"p"`

	// Timestamp is when the message was created (optional, for debugging).
	Timestamp time.Time `json:"timestamp,omitempty" cbor:"t,omitempty"`

	// Metadata holds optional key-value pairs for application use.
	Metadata map[string]string `json:"metadata,omitempty" cbor:"m,omitempty"`
}

// NewMessage creates a new message with the current format version.
//
// The clock is copied to prevent mutation after message creation.
func NewMessage(senderID string, clk *clock.Clock, payload []byte) (*Message, error) {
	if err := validateSenderID(senderID); err != nil {
		return nil, err
	}

	if clk == nil {
		return nil, ErrNilClock
	}

	if len(payload) > MaxPayloadSize {
		return nil, ErrPayloadTooLarge
	}

	return &Message{
		Version:   CurrentVersion,
		SenderID:  senderID,
		Clock:     clk.Copy(),
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}, nil
}

// Validate checks if the message is well-formed and safe to process.
//
// This should be called on all messages from untrusted sources.
func (m *Message) Validate() error {
	if m == nil {
		return errors.New("nil message")
	}

	// Check version
	if m.Version == 0 || m.Version > CurrentVersion {
		return fmt.Errorf("%w: %d", ErrInvalidVersion, m.Version)
	}

	// Validate sender ID
	if err := validateSenderID(m.SenderID); err != nil {
		return err
	}

	// Check clock
	if m.Clock == nil {
		return ErrNilClock
	}

	// Validate payload size
	if len(m.Payload) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}

	// Validate metadata
	if m.Metadata != nil {
		for key, value := range m.Metadata {
			if len(key) > 256 || len(value) > 1024 {
				return errors.New("metadata key or value too large")
			}
		}
	}

	return nil
}

// Size returns the estimated size of the message in bytes.
// This is an approximation used for buffer allocation and monitoring.
func (m *Message) Size() int {
	size := 1 // version
	size += len(m.SenderID)
	size += len(m.Payload)
	size += 8 // timestamp (int64)

	// More accurate clock size estimation
	if m.Clock != nil {
		for _, node := range m.Clock.Nodes() {
			size += len(string(node)) + 8 // NodeID string + int64 value
		}
		size += 16 * m.Clock.Len() // Map overhead per entry
	}

	// Metadata size
	for key, value := range m.Metadata {
		size += len(key) + len(value) + 16 // Key + value + map overhead
	}

	return size
}

// WithMetadata adds metadata to the message (chainable).
// 
// CONCURRENCY WARNING: This method is NOT safe for concurrent use.
// Call this method only during message construction, before sharing
// the message across goroutines.
func (m *Message) WithMetadata(key, value string) *Message {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
	return m
}

// validateSenderID checks if a sender ID is valid.
func validateSenderID(id string) error {
	if id == "" {
		return fmt.Errorf("%w: empty", ErrInvalidSenderID)
	}

	if len(id) > MaxSenderIDLength {
		return fmt.Errorf("%w: too long (%d > %d)", ErrInvalidSenderID, len(id), MaxSenderIDLength)
	}

	// Check for control characters
	for _, r := range id {
		if r < 32 || r == 127 {
			return fmt.Errorf("%w: contains control characters", ErrInvalidSenderID)
		}
	}

	return nil
}
