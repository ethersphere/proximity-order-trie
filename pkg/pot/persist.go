package pot

import (
	"encoding/binary"

	"github.com/nugaon/proximity-order-trie/pkg/persister"
)

var _ Node = (*DBNode)(nil)
var _ persister.TreeNode = (*DBNode)(nil)

const branches = uint32(4) // length of bytes that stores entry length and descendent count

// DBNode extends MemNode with I/O persistence
type DBNode struct {
	*MemNode
	ref  []byte
	newf func() Entry
}

// Empty returns true if no entry is pinned to the Node
func (n *DBNode) Empty() bool {
	return n.MemNode == nil && n.ref == nil || n.MemNode.Empty()
}

// Reference returns the reference to the node to be used to load&unpack the node from disk storage
func (n *DBNode) Reference() []byte {
	return n.ref
}

// SetReference sets the reference to the node to be used to load&unpack the node from disk storage
func (n *DBNode) SetReference(ref []byte) {
	n.ref = ref
}

// Children iterates over children
func (n *DBNode) Children(f func(persister.TreeNode) error) error {
	g := func(cn CNode) (bool, error) {
		return false, f(cn.Node.(*DBNode))
	}
	return n.Iterate(0, g)
}

// MarshalBinary makes DBNode implement the binary.Marshaler interface
// first 4 bytes is the entry length
// next bytes are the entry
// then it appends the forks: 1 byte for PO, then reference (swarm hash), 4 bytes for descendent count
func (n *DBNode) MarshalBinary() ([]byte, error) {
	if Empty(n) || n.Entry() == nil {
		return nil, nil
	}
	entry, err := n.Entry().MarshalBinary()
	if err != nil {
		return nil, err
	}
	l := len(entry)
	buf := make([]byte, l+int(branches))
	binary.BigEndian.PutUint32(buf, uint32(l))
	copy(buf[branches:], entry)
	var sbuf = make([]byte, branches)
	i := 0
	err = n.Iterate(0, func(cn CNode) (bool, error) {
		i++
		buf = append(buf, uint8(cn.At)) // 1 byte for PO(?) why?
		buf = append(buf, cn.Node.(*DBNode).Reference()...)
		binary.BigEndian.PutUint32(sbuf, uint32(cn.Size()))
		buf = append(buf, sbuf...) // it sets cn descendent count.
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	return buf, nil
}

// UnmarshalBinary makes DBNode implement the binary.Unmarshaler interface
// TODO: magic numbers  12, 4, 8 ? -> gives index out of range error
func (n *DBNode) UnmarshalBinary(buf []byte) error {
	// unmarshal entry
	l := binary.BigEndian.Uint32(buf[:branches])
	e := n.newf()
	if err := e.UnmarshalBinary(buf[branches : branches+l]); err != nil {
		return err
	}
	n.Pin(e)
	// unmarshall forks as packed child nodes to be lazy loaded
	buflen := uint32(len(buf))
	for i := branches + l; i < buflen && i+12 < buflen && i+8 < buflen; i += 12 {
		at := int(buf[i])
		m := &DBNode{ref: buf[i+1 : i+8], newf: n.newf}
		size := binary.BigEndian.Uint32(buf[i+8 : i+12])
		cn := CNode{at, m, int(size)}
		n.Append(cn)
	}
	return nil
}
