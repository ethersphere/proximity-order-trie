package elements

import (
	"encoding/binary"

	"github.com/nugaon/proximity-order-trie/pkg/persister"
)

var _ Node = (*SwarmNode)(nil)
var _ persister.TreeNode = (*SwarmNode)(nil)

// SwarmNode extends MemNode with I/O persistence
type SwarmNode struct {
	*MemNode
	ref  []byte
	newf func() Entry
}

// Empty returns true if no entry is pinned to the Node
func (n *SwarmNode) Empty() bool {
	return n.MemNode == nil && n.ref == nil || n.MemNode.Empty()
}

// Reference returns the reference
func (n *SwarmNode) Reference() []byte {
	return n.ref
}

// SetReference sets the reference to the node to be used to load&unpack the node from disk storage
func (n *SwarmNode) SetReference(ref []byte) {
	n.ref = ref
}

// Children iterates over children
func (n *SwarmNode) Children(f func(persister.TreeNode) error) error {
	g := func(cn CNode) (bool, error) {
		return false, f(cn.Node.(*SwarmNode))
	}
	return n.Iterate(0, g)
}

// MarshalBinary makes SwarmNode implement the binary.Marshaler interface
func (n *SwarmNode) MarshalBinary() ([]byte, error) {
	if Empty(n) || n.Entry() == nil {
		return nil, nil
	}
	keyBytes := n.Entry().Key()
	valueBytes, err := n.Entry().MarshalBinary()
	if err != nil {
		return nil, err
	}

	// bitMap is a bitmap of the children
	// it is used to store the children in a sparse array
	bitMap := make([]byte, 32)
	valueBytes = append(valueBytes, keyBytes...)

	setBitMap := func(n uint8) {
		bitMap[n/8] |= 1 << (n % 8)
	}

	forRefBytes := make([]byte, 0)
	forkSizesBytes := make([]byte, 0)
	sbuf := make([]byte, 4)
	err = n.Iterate(0, func(cn CNode) (bool, error) {
		setBitMap(uint8(cn.At))
		forRefBytes = append(forRefBytes, cn.Node.(*SwarmNode).Reference()...)
		binary.BigEndian.PutUint32(sbuf, uint32(cn.Size()))
		forkSizesBytes = append(forkSizesBytes, sbuf...)
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return append(
		keyBytes,
		append(bitMap,
			append(forRefBytes,
				append(forkSizesBytes, valueBytes...)...)...)...,
	), nil
}

// UnmarshalBinary makes SwarmNode implement the binary.Unmarshaler interface
func (n *SwarmNode) UnmarshalBinary(buf []byte) error {
	// reset forks
	n.forks = make([]CNode, 0)

	keyBytes := buf[:32]
	bitMap := buf[32:64]
	frLength := 32
	c := 0
	poMap := make([]int8, 0, 32)
	for i := 0; i < 256; i++ {
		if bitMap[i/8]&(1<<(i%8)) != 0 {
			poMap = append(poMap, int8(i))
			c++
		}
	}

	// unmarshall forks as packed child nodes to be lazy loaded
	for i := 0; i < c; i++ {
		forkRef := buf[64+i*frLength : 64+(i+1)*frLength]
		size := binary.BigEndian.Uint32(buf[64+c*frLength+i*4 : 64+c*frLength+(i+1)*4])
		cn := CNode{
			At:   int(poMap[i]),
			Node: &SwarmNode{ref: forkRef, newf: n.newf},
			size: int(size),
		}
		n.Append(cn)
	}

	// pin entry
	offset := 64 + c*frLength + c*4
	e := n.newf()
	if err := e.UnmarshalBinary(append(keyBytes, buf[offset:]...)); err != nil {
		return err
	}
	n.Pin(e)
	return nil
}
