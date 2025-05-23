package persister_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/ethersphere/proximity-order-trie/pkg/persister"
)

type Loader interface {
	// Load a reference in byte slice representation and return all content associated with the reference.
	Load(context.Context, []byte) ([]byte, error)
}

type Saver interface {
	// Save an arbitrary byte slice and return the reference byte slice representation.
	Save(context.Context, []byte) ([]byte, error)
}

type loadSaver interface {
	Loader
	Saver
}

type mockLoadSaver struct {
	mtx   sync.Mutex
	store map[addr][]byte
}

func newMockLoadSaver() *mockLoadSaver {
	return &mockLoadSaver{
		store: make(map[addr][]byte),
	}
}

func (m *mockLoadSaver) Save(_ context.Context, b []byte) ([]byte, error) {
	var a addr
	hasher := sha256.New()
	_, err := hasher.Write(b)
	if err != nil {
		return nil, err
	}
	copy(a[:], hasher.Sum(nil))
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.store[a] = b
	return a[:], nil
}

func (m *mockLoadSaver) Load(_ context.Context, ab []byte) ([]byte, error) {
	var a addr
	copy(a[:], ab)
	m.mtx.Lock()
	defer m.mtx.Unlock()
	b, ok := m.store[a]
	if !ok {
		return nil, errors.New("not found")
	}
	return b, nil
}

func (m *mockLoadSaver) Close() error {
	return nil
}

type mockTreeNode struct {
	ref      []byte
	children []*mockTreeNode
	val      int
}

func newMockTreeNode(depth, val int) *mockTreeNode {
	mtn := &mockTreeNode{val: val}
	if depth == 0 {
		return mtn
	}
	val <<= branchbits
	for i := 0; i < branches; i++ {
		mtn.children = append(mtn.children, newMockTreeNode(depth-1, val+i))
	}
	return mtn
}

func (mtn *mockTreeNode) Reference() []byte {
	return mtn.ref
}

func (mtn *mockTreeNode) SetReference(b []byte) {
	mtn.ref = b
}

func (mtn *mockTreeNode) Children(f func(persister.TreeNode) error) error {
	for _, n := range mtn.children {
		if err := f(n); err != nil {
			return err
		}
	}
	return nil
}

func (mtn *mockTreeNode) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(mtn.val))
	for _, ch := range mtn.children {
		buf = append(buf, ch.Reference()...)
	}
	return buf, nil
}

func (mtn *mockTreeNode) UnmarshalBinary(buf []byte) error {
	mtn.val = int(binary.BigEndian.Uint32(buf[:4]))
	for i := branches; i < len(buf); i += 32 {
		mtn.children = append(mtn.children, &mockTreeNode{ref: buf[i : i+32]})
	}
	return nil
}
