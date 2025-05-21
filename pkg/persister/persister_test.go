package persister_test

import (
	"context"
	"testing"

	"github.com/nugaon/proximity-order-trie/pkg/persister"
)

const (
	branchbits = 2
	branches   = 4
	depth      = 3
)

type addr [32]byte

func TestPersistIdempotence(t *testing.T) {
	var ls loadSaver = newMockLoadSaver()
	n := newMockTreeNode(depth, 1)
	ctx := context.Background()
	err := persister.Save(ctx, ls, n)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	root := &mockTreeNode{ref: n.Reference()}
	sum := 1
	base := 1
	for i := 0; i < depth; i++ {
		base *= branches
		sum += base
	}
	if c := loadAndCheck(t, ls, root, 1); c != sum {
		t.Fatalf("incorrect nodecount. want 85, got %d", sum)
	}
}

func loadAndCheck(t *testing.T, ls loadSaver, n *mockTreeNode, val int) int {
	t.Helper()
	ctx := context.Background()
	if err := persister.Load(ctx, ls, n); err != nil {
		t.Fatal(err)
	}
	if n.val != val {
		t.Fatalf("incorrect value. want %d, got %d", val, n.val)
	}
	val <<= branchbits
	c := 1
	for i, ch := range n.children {
		c += loadAndCheck(t, ls, ch, val+i)
	}
	return c
}
