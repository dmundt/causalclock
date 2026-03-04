package message

import "sync"

// Serializer defines the interface for message serialization.
//
// Implementations must be safe for concurrent use and handle malformed input.
type Serializer interface {
	// Marshal serializes a message to bytes.
	// Returns ErrMessageTooLarge if the result exceeds MaxMessageSize.
	Marshal(msg *Message) ([]byte, error)

	// Unmarshal deserializes bytes to a message.
	// Must validate the message before returning - returns validation errors.
	Unmarshal(data []byte) (*Message, error)

	// Name returns the serializer name (e.g., "json", "cbor").
	Name() string
}

// Registry manages available serializers with concurrency-safe access.
type Registry struct {
	mu          sync.RWMutex
	serializers map[string]Serializer
}

// NewRegistry creates a new serializer registry.
func NewRegistry() *Registry {
	return &Registry{
		serializers: make(map[string]Serializer),
	}
}

// Register adds a serializer to the registry.
// Safe for concurrent use.
func (r *Registry) Register(s Serializer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serializers[s.Name()] = s
}

// Get retrieves a serializer by name.
// Safe for concurrent use.
func (r *Registry) Get(name string) (Serializer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.serializers[name]
	return s, ok
}

// DefaultRegistry contains the default serializers.
var DefaultRegistry = NewRegistry()

func init() {
	DefaultRegistry.Register(&JSONSerializer{})

	// CBORSerializer requires initialization
	if cbor, err := NewCBORSerializer(); err == nil {
		DefaultRegistry.Register(cbor)
	}
	// If CBOR initialization fails, it simply won't be available in the default registry
}
