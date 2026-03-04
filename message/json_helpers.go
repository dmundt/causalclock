package message

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dmundt/causalclock/clock"
)

// messageJSON is a helper struct for JSON serialization that includes clock data.
type messageJSON struct {
	Version   uint8             `json:"version"`
	SenderID  string            `json:"sender_id"`
	ClockData map[string]int64  `json:"clock"`
	Payload   []byte            `json:"payload"`
	Timestamp int64             `json:"timestamp,omitempty"` // Unix epoch
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// MarshalJSON implements custom JSON marshaling for Message.
func (m *Message) MarshalJSON() ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("cannot marshal nil message")
	}

	// Convert clock to map
	clockData := make(map[string]int64)
	if m.Clock != nil {
		for _, node := range m.Clock.Nodes() {
			clockData[string(node)] = m.Clock.Get(node)
		}
	}

	helper := messageJSON{
		Version:   m.Version,
		SenderID:  m.SenderID,
		ClockData: clockData,
		Payload:   m.Payload,
		Metadata:  m.Metadata,
	}

	if !m.Timestamp.IsZero() {
		helper.Timestamp = m.Timestamp.Unix()
	}

	return json.Marshal(helper)
}

// UnmarshalJSON implements custom JSON unmarshaling for Message.
func (m *Message) UnmarshalJSON(data []byte) error {
	var helper messageJSON
	
	// Use decoder with strict mode to detect unknown fields
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	
	if err := decoder.Decode(&helper); err != nil {
		return err
	}

	// Check for nil clock data (null in JSON)
	if helper.ClockData == nil {
		return fmt.Errorf("clock data cannot be null")
	}

	// Reconstruct clock from map
	clk := clock.NewClock()
	for node, value := range helper.ClockData {
		if value < 0 {
			return fmt.Errorf("invalid clock value for node %s: %d", node, value)
		}
		
		// Set the value by merging with a temporary clock
		tempClock := clock.NewClock()
		for i := int64(0); i < value; i++ {
			tempClock.Increment(clock.NodeID(node))
		}
		clk.Merge(tempClock)
	}

	m.Version = helper.Version
	m.SenderID = helper.SenderID
	m.Clock = clk
	m.Payload = helper.Payload
	m.Metadata = helper.Metadata

	if helper.Timestamp > 0 {
		m.Timestamp = time.Unix(helper.Timestamp, 0)
	}

	return nil
}
