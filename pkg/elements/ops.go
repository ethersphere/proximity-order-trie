package elements

import "context"

const MaxDepth = 256

/*
Wedge is used when a new node must be created between existing nodes.
1. The first node's children between its position and the second node's position
2. The second node itself (if not empty)
3. The first node's children after the second node's position
4. Pins the first node's entry to the result
*/
func Wedge(acc Node, n, m CNode) {
	Append(acc, n.Node, n.At, m.At)
	if !Empty(m.Node) {
		acc.Append(m)
	}
	Append(acc, n.Node, m.At+1, MaxDepth)
	acc.Pin(n.Node.Entry())
}

/*
Whirl is used when integrating a new node into an existing structure.
1. Appends the first node's children up to the second node's position
2. Adds the first node at the second node's position
3. Pins the second node's entry to the result
*/
func Whirl(acc Node, n, m CNode) {
	Append(acc, n.Node, n.At, m.At)
	acc.Append(NewAt(m.At, n.Node))
	acc.Pin(m.Node.Entry())
}

/*
Whack is used for merging operations, often when replacing a node with a different one.
1. Appends the first node's children up to the second node's position
2. Adds the first node at the second node's position (if not at maximum depth)
3. Appends the second node's children from its position onwards
4. Pins the second node's entry to the result
*/
func Whack(acc Node, n, m CNode) {
	Append(acc, n.Node, n.At, m.At)
	if m.At < MaxDepth {
		acc.Append(NewAt(m.At, n.Node))
	}
	Append(acc, m.Node, m.At+1, MaxDepth)
	acc.Pin(m.Node.Entry())
}

// Update
func Update(ctx context.Context, acc Node, cn CNode, k []byte, eqf func(Entry) Entry, mode Mode) (Node, error) {
	u, err := update(ctx, acc, cn, k, eqf, mode)
	if err != nil {
		return nil, err
	}
	if err := mode.Pack(u); err != nil {
		return nil, err
	}
	return u, nil
}

// what `eqf` does(?)
func update(ctx context.Context, acc Node, cn CNode, k []byte, eqf func(Entry) Entry, mode Mode) (Node, error) {
	/**
	1. **Empty node case**: If target node is empty, simply pin the new entry
	2. **Exact match case**: Update the entry if needed and use Whack to rebuild
	3. **Empty match case**: Create a new node with the entry and use Whirl
	4. **Special cases for proximity**: Different combinations of operations based on the node's depth and the Mode's preferences
	5. **Recursive descent**: Using the Mode's policy to determine when to recurse into the trie
	**/
	if Empty(cn.Node) {
		e := eqf(nil)
		if e == nil {
			return nil, nil
		}
		acc.Pin(e)
		return acc, nil
	}
	cm, match, err := FindNext(ctx, cn, k, mode)
	if err != nil {
		return nil, err
	}
	if match {
		orig := cn.Node.Entry()
		entry := eqf(orig)
		if entry == nil {
			node := Pull(acc, cn, mode)
			return node, nil
		}
		if entry.Equal(orig) {
			return nil, nil
		}
		n := mode.New()
		n.Pin(entry)
		Whack(acc, cn, NewAt(mode.Depth(), n))
		return acc, nil
	}
	if Empty(cm.Node) {
		entry := eqf(nil)
		if entry == nil {
			return nil, nil
		}
		n := mode.New()
		n.Pin(entry)
		Whirl(acc, cn, NewAt(cm.At, n))
		return acc, nil
	}
	if cm.At == 0 {
		res, err := update(ctx, acc, cm, k, eqf, mode)
		if err != nil {
			return nil, err
		}
		cm := NewAt(-1, res)
		if cm.Node == nil {
			Wedge(acc, cn, NewAt(0, cm.Node))
			return acc, nil
		}
		if mode.Down(cm) {
			acc := mode.New()
			Wedge(acc, cn, cm)
			return acc, nil
		}
		n := mode.New()
		Whack(n, cm, cn)
		return n, nil
	}
	if mode.Down(cm) {
		res, err := update(ctx, mode.New(), cm, k, eqf, mode)
		if err != nil {
			return nil, err
		}
		Wedge(acc, cn, NewAt(cm.At, res))
		return acc, nil
	}
	Whirl(acc, cn, cm)
	return update(ctx, acc, cm.Next(), k, eqf, mode)
}

// Pull handles node removal and restructuring of the trie.
func Pull(acc Node, cn CNode, mode Mode) Node {
	if f := mode.Up(); f == nil {
		cm := FindFork(cn, nil, mode)
		if !Empty(cm.Node) {
			Wedge(acc, cn, NewAt(cm.At, nil))
			return pullTail(acc, cm.Next(), mode)
		}
		j := cn.At - 1
		cn = acc.Fork(j)
		acc.Truncate(j)
		if cn.Node == nil {
			// this happens only if the pot is singleton
			return mode.New()
		}
		Wedge(acc, cn, NewAt(j, nil))
		return acc
	}
	return pull(acc, cn, mode)
}

func pull(_ Node, _ CNode, _ Mode) Node {
	return nil
}

func pullTail(acc Node, cn CNode, mode Mode) Node {
	cm := FindFork(cn, nil, mode)
	if Empty(cm.Node) {
		Wedge(acc, cn, NewAt(mode.Depth(), nil))
		return acc
	}
	Whirl(acc, cn, cm)
	return pullTail(acc, cm.Next(), mode)
}
