package pot

import (
	"bytes"
	"fmt"

	"github.com/ethersphere/proximity-order-trie/pkg/elements"
)

// Entry implements pot Entry
var _ elements.Entry = (*SwarmEntry)(nil)

type SwarmEntry struct {
	key []byte
	val []byte
}

// NewSwarmEntry on returns an Entry from the given key and value
func NewSwarmEntry(key []byte, val []byte) (*SwarmEntry, error) {
	return &SwarmEntry{
		key: key,
		val: val,
	}, nil
}

func (e *SwarmEntry) Key() []byte {
	return e.key
}

func (e *SwarmEntry) Value() []byte {
	return e.val
}

func (e *SwarmEntry) String() string {
	return fmt.Sprintf("key: %x; val: %v", e.key, e.val)
}

func (e *SwarmEntry) Equal(v elements.Entry) bool {
	ev, ok := v.(*SwarmEntry)
	if !ok {
		return false
	}
	return bytes.Equal(e.val, ev.val)
}

func (e *SwarmEntry) MarshalBinary() ([]byte, error) {
	return e.val, nil
}

func (e *SwarmEntry) UnmarshalBinary(v []byte) error {
	e.val = v
	return nil
}
