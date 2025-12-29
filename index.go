package pot

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethersphere/proximity-order-trie/pkg/elements"
)

// Index represents a mutable pot
type Index struct {
	mode  elements.Mode      // mode
	read  chan elements.Node // hands out current root for reads
	write chan elements.Node // hands out current root for writes and locks
	root  chan elements.Node // channel for new roots
	quit  chan struct{}      // closing this channel signals quit
	closed bool
}

// New constructs a new mutable pot
func New(mode elements.Mode) (*Index, error) {
	idx := &Index{
		mode:  mode,
		read:  make(chan elements.Node),
		write: make(chan elements.Node),
		root:  make(chan elements.Node),
		quit:  make(chan struct{}),
		closed: false,
	}

	root := idx.mode.New()
	go idx.muxProcess(root)
	return idx, nil
}

// NewReference constructs a new mutable pot from a reference
func NewReference(ctx context.Context, mode elements.Mode, ref []byte) (*Index, error) {
	idx := &Index{
		mode:  mode,
		read:  make(chan elements.Node),
		write: make(chan elements.Node),
		root:  make(chan elements.Node),
		quit:  make(chan struct{}),
		closed: false,
	}

	root, loaded, err := idx.mode.Load(ctx, ref)
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
func (idx *Index) muxProcess(root elements.Node) {
	write := idx.write
	quit := idx.quit
	for {
		select {
		case <-quit:
			idx.closed = true
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
func (idx *Index) Add(ctx context.Context, e elements.Entry) error {
	return idx.Update(ctx, e.Key(), func(elements.Entry) elements.Entry { return e })
}

// Delete removes the entry at the given key from the mutable pot
func (idx *Index) Delete(ctx context.Context, k []byte) error {
	return idx.Update(ctx, k, func(elements.Entry) elements.Entry { return nil })
}

// Update exposes the pot update function more directly
func (idx *Index) Update(ctx context.Context, k []byte, f func(elements.Entry) elements.Entry) error {
	var root elements.Node

	if idx.closed {
		return errors.New("trie closed")
	}

	// get the pot root and capture the write lock
	select {
	case <-ctx.Done():
		return ctx.Err()
	case root = <-idx.write:
	}

	update, err := idx.mode.Update(ctx, root, k, f)
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

// Find retrieves the entry at the given key from the mutable pot or gives elements.ErrNotFound
func (idx *Index) Find(ctx context.Context, k []byte) (elements.Entry, error) {

	if idx.closed {
		return nil, errors.New("trie closed")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case root := <-idx.read:
		return elements.Find(ctx, root, k, idx.mode)
	}
}

// Iterate wraps the underlying pot's iterator
func (idx *Index) Iterate(ctx context.Context, p, k []byte, f func(elements.Entry) (stop bool, err error)) error {
	if idx.closed {
		return errors.New("trie closed")
	}
	return elements.Iterate(ctx, elements.NewAt(0, <-idx.read), p, k, idx.mode, f)
}

// Size returns the size (number of entries) of the pot
func (idx *Index) Size() (int, error) {
	if idx.closed {
		return 0, errors.New("trie closed")
	}
	root := <-idx.read
	if root == nil {
		return 0, nil
	}
	return root.Size(), nil
}

// Save calls the mode specific save method for the root node
func (idx *Index) Save(ctx context.Context) ([]byte, error) {
	if idx.closed {
		return nil, errors.New("trie closed")
	}
	root := <-idx.read
	if root.Empty() {
		return nil, fmt.Errorf("root node is nil")
	}
	return idx.mode.Save(ctx)
}

// Close quits the process loop and closes the mode
func (idx *Index) Close() error {
	if idx.closed {
		return errors.New("trie closed")
	}
	close(idx.quit)
	idx.closed = true
	return nil
}

// String pretty prints the current state of the pot
func (idx *Index) String() (string, error) {
	if idx.closed {
		return "", errors.New("trie closed")
	}
	root := <-idx.read
	return elements.NewAt(0, root).String(), nil
}
