package proximityordertrie

import (
	"bytes"
	"fmt"

	"github.com/nugaon/proximity-order-trie/pkg/pot"
)

// Entry implements pot Entry
var _ pot.Entry = (*SwarmEntry)(nil)

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

func (e *SwarmEntry) Equal(v pot.Entry) bool {
	ev, ok := v.(*SwarmEntry)
	if !ok {
		return false
	}
	return bytes.Equal(e.val, ev.val)
}

func (e *SwarmEntry) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 32+len(e.val))
	copy(buf[:32], e.key)
	copy(buf[32:], e.val)
	return buf, nil
}

func (e *SwarmEntry) UnmarshalBinary(v []byte) error {
	if len(v) < 32 {
		return fmt.Errorf("invalid entry size: %d", len(v))
	}

	copy(e.key, v[:32])
	copy(e.val, v[32:])
	return nil
}
