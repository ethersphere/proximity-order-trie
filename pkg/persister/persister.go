package persister

import (
	"context"
	"encoding"
	"fmt"

	"golang.org/x/crypto/sha3"
)

// LoadSaver to be implemented as thin wrappers around persistent key-value storage
type LoadSaver interface {
	Load(ctx context.Context, reference []byte) (data []byte, err error) // retrieve nodes for read only operations only
	Save(ctx context.Context, data []byte) (reference []byte, err error) // persists nodes out of scopc	qfor write operations
}

// TreeNode is a generic interface for recursive persistable data structures
type TreeNode interface {
	Reference() []byte
	SetReference([]byte)
	Children(func(TreeNode) error) error
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

type InmemLoadSaver struct {
	store map[[32]byte][]byte
}

func NewInmemLoadSaver() *InmemLoadSaver {
	return &InmemLoadSaver{
		store: make(map[[32]byte][]byte),
	}
}

func (ls *InmemLoadSaver) Load(ctx context.Context, reference []byte) ([]byte, error) {
	if len(reference) != 32 {
		return nil, fmt.Errorf("reference must be 32 bytes, got %d", len(reference))
	}
	var refArr [32]byte
	copy(refArr[:], reference)
	data, ok := ls.store[refArr]
	if !ok {
		return nil, fmt.Errorf("reference not found")
	}
	return data, nil
}

func (ls *InmemLoadSaver) Save(ctx context.Context, data []byte) ([]byte, error) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	var ref [32]byte
	copy(ref[:], hasher.Sum(nil))
	ls.store[ref] = data
	return ref[:], nil
}

// Load uses a Loader to unmarshal a tree node from a reference
func Load(ctx context.Context, ls LoadSaver, n TreeNode) error {
	b, err := ls.Load(ctx, n.Reference())
	if err != nil {
		return err
	}
	return n.UnmarshalBinary(b)
}

// Save persists a trie recursively traversing the nodes
func Save(ctx context.Context, ls LoadSaver, n TreeNode) error {
	if ref := n.Reference(); len(ref) > 0 {
		return nil
	}
	f := func(tn TreeNode) error {
		return Save(ctx, ls, tn)
	}
	if err := n.Children(f); err != nil {
		return err
	}
	bytes, err := n.MarshalBinary()
	if err != nil {
		return err
	}
	ref, err := ls.Save(ctx, bytes)
	if err != nil {
		return err
	}
	n.SetReference(ref)
	return nil
}
