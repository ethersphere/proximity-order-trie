package proximityordertrie

import (
	"context"
	"fmt"

	"github.com/nugaon/proximity-order-trie/pkg/pot"
)

// Index represents a mutable pot
type Index struct {
	mode  pot.Mode      // mode
	read  chan pot.Node // hands out current root for reads
	write chan pot.Node // hands out current root for writes and locks
	root  chan pot.Node // channel for new roots
	quit  chan struct{} // closing this channel signals quit
}

// New constructs a new mutable pot
func New(mode pot.Mode) (*Index, error) {
	idx := &Index{
		mode:  mode,
		read:  make(chan pot.Node),
		write: make(chan pot.Node),
		root:  make(chan pot.Node),
		quit:  make(chan struct{}),
	}

	root := idx.mode.New()
	go idx.muxProcess(root)
	return idx, nil
}

// New constructs a new mutable pot from a reference
func NewReference(mode pot.Mode, ref []byte) (*Index, error) {
	idx := &Index{
		mode:  mode,
		read:  make(chan pot.Node),
		write: make(chan pot.Node),
		root:  make(chan pot.Node),
		quit:  make(chan struct{}),
	}

	root, loaded, err := idx.mode.Load(context.TODO(), ref)
	if err != nil {
		return nil, err
	}
	if !loaded {
		return nil, fmt.Errorf("root not loaded from persistent storage")
	}
	go idx.muxProcess(root)
	return idx, nil
}

// muxProcess is a forever loop serving as a locking mechanism for the pot index
// it allows only a single write operation at a time but multiple reads
func (idx *Index) muxProcess(root pot.Node) {
	write := idx.write
	quit := idx.quit
	for {
		select {
		case <-quit:
			return
		case idx.read <- root: //
		case write <- root: // write locks the pot for writes
			write = nil // locks the pot until root updated
			quit = nil  // disallow quit until write finish
		case root = <-idx.root:
			write = idx.write
			quit = idx.quit
		}
	}
}

// Add inserts an entry to the mutable pot
func (idx *Index) Add(ctx context.Context, e pot.Entry) error {
	return idx.Update(ctx, e.Key(), func(pot.Entry) pot.Entry { return e })
}

// Delete removes the entry at the given key from the mutable pot
func (idx *Index) Delete(ctx context.Context, k []byte) error {
	return idx.Update(ctx, k, func(pot.Entry) pot.Entry { return nil })
}

// Update exposes the pot update function more directly
func (idx *Index) Update(ctx context.Context, k []byte, f func(pot.Entry) pot.Entry) error {
	var root pot.Node

	// get the pot root and capture the write lock
	select {
	case <-ctx.Done():
		return ctx.Err()
	case root = <-idx.write:
	}

	update, err := idx.mode.Update(root, k, f)
	if err != nil {
		return err
	}
	if update != nil {
		root = update
	}

	// update with new pot root and release the write lock
	select {
	case <-ctx.Done():
		return ctx.Err()
	case idx.root <- root:
	}
	return nil
}

// Find retrieves the entry at the given key from the mutable pot or gives pot.ErrNotFound
func (idx *Index) Find(ctx context.Context, k []byte) (pot.Entry, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case root := <-idx.read:
		return pot.Find(root, k, idx.mode)
	}
}

// Iterate wraps the underlying pot's iterator
func (idx *Index) Iterate(p, k []byte, f func(pot.Entry) (stop bool, err error)) error {
	return pot.Iterate(pot.NewAt(0, <-idx.read), p, k, idx.mode, f)
}

// Size returns the size (number of entries) of the pot
func (idx *Index) Size() int {
	root := <-idx.read
	if root == nil {
		return 0
	}
	return root.Size()
}

// Save calls the mode specific save method for the root node
func (idx *Index) Save(ctx context.Context) ([]byte, error) {
	root := <-idx.read
	if root == nil {
		return nil, fmt.Errorf("root node is nil")
	}
	return idx.mode.Save(ctx)
}

// String pretty prints the current state of the pot
func (idx *Index) String() string {
	root := <-idx.read
	return pot.NewAt(0, root).String()
}
