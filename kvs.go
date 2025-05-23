package proximityordertrie

import (
	"context"
	"errors"
	"fmt"

	"github.com/nugaon/proximity-order-trie/pkg/persister"
	"github.com/nugaon/proximity-order-trie/pkg/pot"
)

var _ KeyValueStore = (*swarmKvs)(nil)

var (
	ErrNotFound = errors.New("not found")
)

// KeyValueStore represents a key-value store.
type KeyValueStore interface {
	// Get retrieves the value associated with the given key.
	Get(ctx context.Context, key []byte) ([]byte, error)
	// Put stores the given key-value pair in the store.
	Put(ctx context.Context, key, value []byte) error
	// Save saves key-value pair to the underlying storage and returns the reference.
	Save(ctx context.Context) ([]byte, error)
}

type swarmKvs struct {
	idx *Index
}

// NewSwarmKvs creates a new key-value store with pot as the underlying storage.
func NewSwarmKvs(ls persister.LoadSaver) (*swarmKvs, error) {
	basePotMode := pot.NewSingleOrder(256)
	mode := pot.NewSwarmPot(basePotMode, ls, func() pot.Entry { return &SwarmEntry{} })
	idx, err := New(mode)
	if err != nil {
		return nil, fmt.Errorf("failed to create pot: %w", err)
	}

	return &swarmKvs{
		idx: idx,
	}, nil
}

// NewSwarmKvsReference loads a key-value store from the given root hash with pot as the underlying storage.
func NewSwarmKvsReference(ls persister.LoadSaver, ref []byte) (*swarmKvs, error) {
	basePotMode := pot.NewSingleOrder(256)
	mode := pot.NewPersistedPotReference(basePotMode, ls, ref, func() pot.Entry { return &SwarmEntry{} })
	idx, err := NewReference(mode, ref)
	if err != nil {
		return nil, fmt.Errorf("failed to create pot reference: %w", err)
	}

	return &swarmKvs{
		idx: idx,
	}, nil
}

// Get retrieves the value associated with the given key.
func (ps *swarmKvs) Get(ctx context.Context, key []byte) ([]byte, error) {
	entry, err := ps.idx.Find(ctx, key)
	if err != nil {
		return nil, err
	}

	return entry.(*SwarmEntry).Value(), nil
}

// Put stores the given key-value pair in the store.
func (ps *swarmKvs) Put(ctx context.Context, key []byte, value []byte) error {
	entry, err := NewSwarmEntry(key, value)
	if err != nil {
		return err
	}
	err = ps.idx.Add(ctx, entry)
	if err != nil {
		return fmt.Errorf("failed to put value to pot %w", err)
	}
	return nil
}

// Save saves key-value pair to the underlying storage and returns the reference.
func (ps *swarmKvs) Save(ctx context.Context) ([]byte, error) {
	ref, err := ps.idx.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to store pot %w", err)
	}
	return ref, nil
}
