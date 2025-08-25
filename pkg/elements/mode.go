package elements

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ethersphere/proximity-order-trie/pkg/persister"
)

type Mode interface {
	Depth() int                                                            // maximum bit length of key
	New() Node                                                             // constructor
	Pack(ctx context.Context, n Node) error                                // mode specific saving of a node
	Unpack(ctx context.Context, n Node) error                              // mode specific loading of a node
	Down(CNode) bool                                                       // dictates insertion policy
	Up() func(CNode) bool                                                  // dictates which node/entry to promote after deletion
	Load(context.Context, []byte) (Node, bool, error)                      // loads the pot
	Save(context.Context) ([]byte, error)                                  // saves the pot
	Update(context.Context, Node, []byte, func(Entry) Entry) (Node, error) // mode specific update
}

type SingleOrder struct {
	depth int
}

var _ Mode = (*SingleOrder)(nil)

func NewSingleOrder(d int) *SingleOrder {
	return &SingleOrder{depth: d}
}

// Pack NOOP
func (SingleOrder) Pack(ctx context.Context, n Node) error {
	return nil
}

// Unpack NOOP
func (SingleOrder) Unpack(ctx context.Context, n Node) error {
	return nil
}

// Down dictates insert policy - NOOP
func (SingleOrder) Down(_ CNode) bool {
	return false
}

// Up dictates choice for promoting nodes after deletion  - NOOP
func (SingleOrder) Up() func(CNode) bool {
	return nil
}

// New constructs a new in-memory Node
func (SingleOrder) New() Node {
	return &MemNode{}
}

// Depth returns the length of a key
func (s SingleOrder) Depth() int {
	return s.depth
}

// Save NOOP
func (SingleOrder) Save(context.Context) ([]byte, error) {
	return nil, nil
}

// Load NOOP
func (so SingleOrder) Load(context.Context, []byte) (Node, bool, error) {
	return so.New(), false, nil
}

// Update is mode specific pot update function - NOOP just proxies to pkg wide default
func (so SingleOrder) Update(ctx context.Context, root Node, k []byte, f func(Entry) Entry) (Node, error) {
	return Update(ctx, so.New(), NewAt(0, root), k, f, so)
}

// Mode for Swarm persisted pots
type SwarmPot struct {
	Mode                        // non-persisted mode
	n    Node                   // root node
	ls   persister.LoadSaver    // persister interface to save pointer based data structure nodes
	newf func(key []byte) Entry // pot entry constructor function. Entry must set the given key
}

// NewSwarmPot constructs a Mode for persisted pots
func NewSwarmPot(mode Mode, ls persister.LoadSaver, newf func(key []byte) Entry) *SwarmPot {
	return &SwarmPot{Mode: mode, n: &SwarmNode{newf: newf, MemNode: &MemNode{}}, ls: ls, newf: newf}
}

// NewSwarmPotReference constructs a Mode for persisted pots with a reference
func NewSwarmPotReference(mode Mode, ls persister.LoadSaver, ref []byte, newf func(key []byte) Entry) *SwarmPot {
	return &SwarmPot{Mode: mode, n: &SwarmNode{newf: newf, MemNode: &MemNode{}, ref: ref}, ls: ls, newf: newf}
}

// newPacked constructs a packed node that allows loading via its reference
func (pm *SwarmPot) NewPacked(ref []byte) *SwarmNode {
	return &SwarmNode{newf: pm.newf, ref: ref}
}

// Load loads the pot by reading the root reference from a file and creating the root node
func (pm *SwarmPot) Load(ctx context.Context, ref []byte) (r Node, loaded bool, err error) {
	root := pm.NewPacked(ref)
	root.MemNode = &MemNode{}
	if err := persister.Load(ctx, pm.ls, root); err != nil {
		return nil, false, fmt.Errorf("failed to load persisted pot root node at %s: %w", hex.EncodeToString(ref), err)
	}
	pm.n = root
	return root, true, nil
}

// Save persists the root node reference
func (pm *SwarmPot) Save(ctx context.Context) ([]byte, error) {
	if pm.n == nil {
		return nil, fmt.Errorf("node is nil")
	}

	err := persister.Save(ctx, pm.ls, pm.n.(*SwarmNode))
	if err != nil {
		return nil, fmt.Errorf("pot save: %w", err)
	}

	return pm.n.(*SwarmNode).Reference(), nil
}

// Update builds on the generic Update
func (pm *SwarmPot) Update(ctx context.Context, root Node, k []byte, f func(Entry) Entry) (Node, error) {
	update, err := Update(ctx, pm.New(), NewAt(0, root), k, f, pm)
	if err != nil {
		return nil, err
	}
	pm.n = update
	return update, nil
}

// Pack serialises and saves the object
// once a new node is saved it can be delinked as node from memory
func (pm *SwarmPot) Pack(ctx context.Context, n Node) error {
	if n == nil {
		return nil // nothing to save
	}
	return persister.Save(ctx, pm.ls, n.(*SwarmNode))
}

// Unpack loads and deserialises node into memory
func (pm *SwarmPot) Unpack(ctx context.Context, n Node) error {
	if n == nil {
		return nil
	}
	dn := n.(*SwarmNode)
	if dn.MemNode != nil {
		return nil
	}
	dn.MemNode = &MemNode{}
	return persister.Load(ctx, pm.ls, dn)
}

// New constructs a new node
func (pm *SwarmPot) New() Node {
	return &SwarmNode{newf: pm.newf, MemNode: &MemNode{}}
}
