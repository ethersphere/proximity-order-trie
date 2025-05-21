package pot

import (
	"encoding"
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrNotFound = errors.New("not found")
)

type Entry interface {
	Key() []byte
	Equal(Entry) bool
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	fmt.Stringer
}

// Node is an interface for pot nodes
// implementations
type Node interface {
	Pin(Entry)                                           // pin an entry to the node
	Entry() Entry                                        // get entry pinned to the Node
	Empty() bool                                         // get entry pinned to the Node
	Size() int                                           // get number of entries in the pot
	fmt.Stringer                                         // pretty print
	Fork(po int) CNode                                   // get child node fork at PO po
	Append(CNode)                                        // append child node to forks
	Truncate(i int)                                      // truncate fork list
	Iterate(from int, f func(CNode) (bool, error)) error // iterate over children starting at PO from
}

// CNode captures a child node (or cursor+node), a node viewed as a fork of a parent node
type CNode struct {
	At   int // PO(?)
	Node Node
	size int
}

// NewAt creates a cursored node and recalculates the size as the sum of sizes of its children
func NewAt(at int, n Node) CNode {
	if Empty(n) {
		// if n == nil || n.Size() == 0 {
		return CNode{at, nil, 0}
	}
	size := 1
	_ = n.Iterate(at+1, func(c CNode) (bool, error) {
		size += c.size
		return false, nil
	})
	return CNode{at, n, size}
}

// Next returns a CNode, that is the view of the same Node from a po following the At of the receiver CNode
func (c CNode) Next() CNode {
	// return NewAt(c.At+1, c.Node)
	n := c.Node.Fork(c.At)
	return CNode{c.At + 1, c.Node, c.Size() - n.Size()}
}

// Size returns the number of entries (=Nodes) subsumed under the node
func (n CNode) Size() int {
	return n.size
}

func KeyOf(n Node) []byte {
	if Empty(n) {
		return nil
	}
	return n.Entry().Key()
}

func Label(k []byte) string {
	if len(k) == 0 {
		return "none"
	}
	return fmt.Sprintf("%032b", binary.BigEndian.Uint32(k[:4]))
}

// Empty
func Empty(n Node) bool {
	return n == nil || n.Empty()
}

func Append(b, n Node, from, to int) {
	b.Truncate(from)
	_ = n.Iterate(from, func(k CNode) (bool, error) {
		if k.At < to {
			b.Append(k)
			return false, nil
		}
		return true, nil
	})
}

// Find finds the entry of a key
func Find(n Node, k []byte, mode Mode) (Entry, error) {
	return find(NewAt(0, n), k, mode)
}

func find(n CNode, k []byte, mode Mode) (Entry, error) {
	if Empty(n.Node) {
		return nil, ErrNotFound
	}
	m, match := FindNext(n, k, mode)
	if match {
		return n.Node.Entry(), nil
	}
	return find(m, k, mode)
}

// Iterate is an iterator that walks all the entries subsumed under the given CNode
// in ascending order of distance from a given key
func Iterate(n CNode, p, k []byte, mode Mode, f func(Entry) (bool, error)) error {
	m, _ := findNode(n, p, mode)
	if Empty(m.Node) {
		return nil
	}
	_, err := iterate(m, k, m.At, mode, f)
	return err
}

func iterate(n CNode, k []byte, at int, mode Mode, f func(Entry) (bool, error)) (stop bool, err error) {
	if Empty(n.Node) {
		return false, nil
	}
	if n.Size() == 1 {
		return f(n.Node.Entry())
	}
	var cn CNode
	po := Compare(n.Node, k, n.At+1)
	cn = n.Node.Fork(po)
	if err := mode.Unpack(cn.Node); err != nil {
		panic(err.Error())
	}
	forks := append(Slice(n.Node, n.At+1, cn.At), NewAt(cn.At, n.Node), cn)
	for i := len(forks) - 1; !stop && err == nil && i >= 0; i-- {
		stop, err = iterate(forks[i], k, at, mode, f)
	}
	return stop, err
}

func Slice(n Node, from, to int) (forks []CNode) {
	_ = n.Iterate(from, func(c CNode) (bool, error) {
		if c.At >= to {
			return true, nil
		}
		forks = append(forks, c)
		return false, nil
	})
	return forks
}

func findNode(n CNode, k []byte, mode Mode) (CNode, error) {
	if Empty(n.Node) {
		return CNode{}, ErrNotFound
	}
	m, ok := FindNext(n, k, mode)
	if ok {
		return NewAt(8*len(k), n.Node), nil
	}
	return findNode(m, k, mode)
}

// FindNext finds the fork on a node that matches the key bytes
func FindNext(n CNode, k []byte, mode Mode) (CNode, bool) {
	po := Compare(n.Node, k, n.At)
	if po < mode.Depth() && po < 8*len(k) {
		cn := n.Node.Fork(po)
		if err := mode.Unpack(cn.Node); err != nil {
			panic(err.Error())
		}
		return cn, false
	}
	return NewAt(mode.Depth(), nil), true
}

// FindFork iterates through the forks on a node and returns the fork
// that returns true when fed to the stop function
// if no stop function is given then returns the last fork
func FindFork(n CNode, f func(CNode) bool, mode Mode) (m CNode) {
	_ = n.Node.Iterate(n.At, func(c CNode) (stop bool, err error) {
		if f == nil {
			m = c
		} else {
			stop = f(m)
		}
		return stop, nil
	})
	return m
}

// Compare compares the key of a CNode with a key, assuming the two match on a prefix of length po
// it returns the proximity order quantifying the distance of the two keys plus
// a boolean second return value which is true if the keys exactly match
func Compare(n Node, k []byte, at int) int {
	return PO(n.Entry().Key(), k, at)
}

// po returns the proximity order of two fixed length byte sequences
// assuming po > pos
func PO(one, other []byte, pos int) int {
	for i := pos / 8; i < len(one) && i < len(other); i++ {
		if one[i] == other[i] {
			continue
		}
		oxo := one[i] ^ other[i]
		start := 0
		if i == pos/8 {
			start = pos % 8
		}
		for j := start; j < 8; j++ {
			if (oxo>>uint8(7-j))&0x01 != 0 {
				return i*8 + j
			}
		}
	}
	return len(other) * 8
}
