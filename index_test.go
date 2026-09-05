package pot_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	pot "github.com/ethersphere/proximity-order-trie"
	"github.com/ethersphere/proximity-order-trie/pkg/elements"
	"github.com/ethersphere/proximity-order-trie/pkg/persister"
	"golang.org/x/sync/errgroup"
	"github.com/stretchr/testify/assert"
)

var basePotMode = elements.NewSingleOrder(256)

type mockEntry struct {
	key []byte
	val int
}

func (m *mockEntry) Key() []byte {
	return m.key
}

func (m *mockEntry) String() string {
	return fmt.Sprintf("%d", m.val)
}

func (m *mockEntry) Equal(n elements.Entry) bool {
	return m.val == n.(*mockEntry).val
}

func eq(m, n *mockEntry) bool {
	return bytes.Equal(m.key, n.key) && m.Equal(n)
}

func (m *mockEntry) MarshalBinary() ([]byte, error) {
	buf := make([]byte, 32)
	binary.BigEndian.PutUint32(buf[28:32], uint32(m.val))
	return buf, nil
}

func (m *mockEntry) UnmarshalBinary(buf []byte) error {
	m.val = int(binary.BigEndian.Uint32(buf[28:32]))
	return nil
}

func TestUpdateCorrectness(t *testing.T) {
	idx, err := pot.New(basePotMode)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	want := newDetMockEntry(t, 0)
	want2 := newDetMockEntry(t, 1)
	ctx := context.Background()
	t.Run("not found on empty index", func(t *testing.T) {
		checkNotFound(t, ctx, idx, want)
	})
	t.Run("add item to empty index and find it", func(t *testing.T) {
		idx.Add(ctx, want)
		checkFound(t, ctx, idx, want)
	})
	t.Run("add same item and find no change", func(t *testing.T) {
		idx.Add(ctx, want)
		checkFound(t, ctx, idx, want)
	})
	t.Run("delete item and not find it", func(t *testing.T) {
		idx.Delete(ctx, want.Key())
		checkNotFound(t, ctx, idx, want)
	})
	t.Run("add 2 items to empty index and find them", func(t *testing.T) {
		idx.Add(ctx, want)
		checkFound(t, ctx, idx, want)
		idx.Add(ctx, want2)
		checkFound(t, ctx, idx, want)
		checkFound(t, ctx, idx, want2)
	})
	t.Run("delete first item and not find it", func(t *testing.T) {
		idx.Delete(ctx, want.Key())
		checkNotFound(t, ctx, idx, want)
		checkFound(t, ctx, idx, want2)
	})
	t.Run("once again add first item and find both", func(t *testing.T) {
		idx.Add(ctx, want)
		checkFound(t, ctx, idx, want2)
		checkFound(t, ctx, idx, want)
	})
	t.Run("delete latest added item and find only item 2", func(t *testing.T) {
		idx.Delete(ctx, want.Key())
		checkFound(t, ctx, idx, want2)
		checkNotFound(t, ctx, idx, want)
	})
	wantMod := &mockEntry{key: want.key, val: want.val + 1}
	want2Mod := &mockEntry{key: want2.key, val: want2.val + 1}
	t.Run("modify item", func(t *testing.T) {
		idx.Add(ctx, want)
		checkFound(t, ctx, idx, want)
		checkFound(t, ctx, idx, want2)
		idx.Add(ctx, wantMod)
		checkFound(t, ctx, idx, wantMod)
		checkFound(t, ctx, idx, want2)
		idx.Add(ctx, want2Mod)
		checkFound(t, ctx, idx, wantMod)
		checkFound(t, ctx, idx, want2Mod)
	})
}

func TestEdgeCasesCorrectness(t *testing.T) {
	ctx := context.Background()
	t.Run("not found on empty index", func(t *testing.T) {
		idx, err := pot.New(basePotMode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()
		ints := []int{0, 1, 2}
		entries := make([]*mockEntry, 3)
		for i, j := range ints {
			entry := newDetMockEntry(t, j)
			idx.Add(ctx, entry)
			entries[i] = entry
		}
		idx.Delete(ctx, entries[1].Key())
		checkNotFound(t, ctx, idx, entries[1])
		checkFound(t, ctx, idx, entries[2])
	})
	t.Run("not found on empty index", func(t *testing.T) {
		idx, err := pot.New(basePotMode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()

		ints := []int{5, 4, 7, 8}
		entries := make([]*mockEntry, 4)
		for i, j := range ints {
			entry := newDetMockEntry(t, j)
			idx.Add(ctx, entry)
			entries[i] = entry
		}
		idx.Delete(ctx, entries[1].Key())
		checkFound(t, ctx, idx, entries[2])
		checkFound(t, ctx, idx, entries[0])
		checkFound(t, ctx, idx, entries[3])
	})
	t.Run("no duplication", func(t *testing.T) {
		idx, err := pot.New(basePotMode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()

		ints := []int{3, 0, 2, 1}
		entries := make([]*mockEntry, 4)
		for i, j := range ints {
			entry := newDetMockEntry(t, j)
			idx.Add(ctx, entry)
			entries[i] = entry
		}
		idx.Delete(ctx, entries[2].key)

		checkFound(t, ctx, idx, entries[0])
		checkFound(t, ctx, idx, entries[1])
		checkFound(t, ctx, idx, entries[3])
		checkNotFound(t, ctx, idx, entries[2])
	})
	t.Run("delete from top", func(t *testing.T) {
		idx, err := pot.New(basePotMode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()

		ints := []int{6, 7}
		entries := make([]*mockEntry, 2)
		for i, j := range ints {
			entry := newDetMockEntry(t, j)
			idx.Add(ctx, entry)
			entries[i] = entry
		}
		idx.Delete(ctx, entries[0].key)
		checkFound(t, ctx, idx, entries[1])
		checkNotFound(t, ctx, idx, entries[0])
	})
}

func TestIterate(t *testing.T) {
	count := 64
	test := func(t *testing.T, idx *pot.Index) {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		pivot := make([]byte, 4)
		for e, b := range []int{0, 256, 512} {
			s := make([]byte, 4)
			binary.BigEndian.PutUint32(s, uint32(b))
			s = s[:3]
			r := make([]int, count)
			for i := range r {
				r[i] = i
			}
			rand.Shuffle(count, func(i, j int) { k := r[i]; r[i] = r[j]; r[j] = k })
			for i := 0; i < count; i++ {
				k := make([]byte, 32)
				binary.BigEndian.PutUint32(k, uint32(b+r[i]))
				e := &mockEntry{k, b + r[i]}
				idx.Add(ctx, e)
				n := 0
				max := 0
				if err := idx.Iterate(ctx, s, pivot, func(e elements.Entry) (bool, error) {
					item := e.(*mockEntry).val
					if max > item {
						t.Fatalf("not ordered correclty: %v > %v", max, item)
					}
					max = item
					n++
					return false, nil
				}); err != nil {
					t.Fatal(err)
				}
				if n != i+1 {
					t.Fatalf("incorrect number of items. want %d, got %d", i+1, n)
				}
			}
			n := 0
			if err := idx.Iterate(ctx, nil, pivot, func(e elements.Entry) (bool, error) {
				n++
				return false, nil
			}); err != nil {
				t.Fatal(err)
			}
			if n != (e+1)*count {
				t.Fatalf("incorrect number of items. want %d, got %d", (e+1)*count, n)
			}
		}
	}
	t.Run("in memory", func(t *testing.T) {
		idx, err := pot.New(elements.NewSingleOrder(32))
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()
		test(t, idx)
	})
	t.Run("persisted", func(t *testing.T) {
		ls := persister.NewInmemLoadSaver()
		mode := elements.NewSwarmPot(elements.NewSingleOrder(32), ls, func(key []byte) elements.Entry { return &mockEntry{key: key} })
		idx, err := pot.New(mode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()
		test(t, idx)
	})
}

// TestIterateWholeStoreUndercount isolates bug 2 from TestIterateAfterLoad's
// doc comment on its own, with no persistence involved at all: it shows the
// undercount is purely a consequence of findNode()'s off-by-one and has
// nothing to do with lazy-loading. 300 pseudo-random (sha256-derived) keys
// make a root-level fork at bit 0 close to certain, which is what the bug
// needs; the existing TestIterate above never creates one because all its
// keys share a fixed 3-byte-range prefix that keeps bit 0 constant.
func TestIterateWholeStoreUndercount(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	count := 300
	idx, err := pot.New(elements.NewSingleOrder(256))
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	for i := 0; i < count; i++ {
		if err := idx.Add(ctx, newDetMockEntry(t, i)); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	pivot := make([]byte, 32)
	if err := idx.Iterate(ctx, nil, pivot, func(e elements.Entry) (bool, error) {
		n++
		return false, nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != count {
		t.Fatalf("want %d entries, got %d (a root-level fork at bit 0 was likely dropped)", count, n)
	}
}

// TestIterateAfterLoad guards against two independent bugs in Iterate(),
// neither of which TestIterate above catches because it always iterates the
// same small (count=64), sequentially-keyed Index instance it just built —
// every node is already resident in memory, and its root never happens to
// fork on the very first bit. Both bugs need a bigger, more realistically
// (pseudo-randomly) keyed pot to surface, which is what this test builds.
//
// Bug 1 — nil pointer dereference on a pot loaded from a reference.
// iterate() unpacked the single fork on the path to the pivot key but handed
// the *other* forks at a node (from Slice()) to its own recursive call
// without unpacking them first. Empty()/Size()/Entry() on such a node
// dereference a nil *MemNode as soon as there is more than one matching key
// below the loaded root.
//
// Bug 2 — silent undercount, independent of persistence. findNode()'s
// "prefix fully matched" branch built the matched node's cursor as
// NewAt(8*len(prefix), node) — one bit too high. Node.Size() itself uses the
// correct convention (NewAt(-1, n) for "nothing fixed yet"): a CNode's At is
// meant to be the position of the *last already-fixed* bit, which for a
// fully-matched prefix of 8*len(prefix) bits is 8*len(prefix)-1, not
// 8*len(prefix). The off-by-one boundary then made Slice()/NewAt() start
// scanning children one bit too late, silently skipping any real fork
// positioned exactly at that boundary bit (bit 0 for the common case of an
// empty/no prefix — an entirely ordinary place for two keys to diverge).
// With 300 pseudo-random keys a root-level fork at bit 0 is close to certain,
// so this test would fail close to half its count without the fix.
//
// This test saves a pot, then opens a *fresh* Index from that save reference
// (mirroring how a real client loads someone else's data) and asserts that
// Iterate over it does not panic and returns every entry, in the documented
// ascending order, both with and without a prefix.
func TestIterateAfterLoad(t *testing.T) {
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	count := 300
	ls := persister.NewInmemLoadSaver()
	newf := func(key []byte) elements.Entry { return &mockEntry{key: key} }
	mode := elements.NewSwarmPot(basePotMode, ls, newf)
	idx, err := pot.New(mode)
	if err != nil {
		t.Fatal(err)
	}
	want := make(map[string]int, count)
	for i := 0; i < count; i++ {
		e := newDetMockEntry(t, i)
		if err := idx.Add(ctx, e); err != nil {
			t.Fatal(err)
		}
		want[string(e.key)] = e.val
	}
	ref, err := idx.Save(ctx)
	if err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// A brand new Index from the reference: every node starts out packed
	// (MemNode == nil), which is exactly the condition the bug needed.
	loadMode := elements.NewSwarmPotReference(basePotMode, ls, ref, newf)
	loaded, err := pot.NewReference(ctx, loadMode, ref)
	if err != nil {
		t.Fatal(err)
	}
	defer loaded.Close()

	t.Run("whole store, no prefix", func(t *testing.T) {
		got := make(map[string]int, count)
		var order [][]byte
		pivot := make([]byte, 32)
		if err := loaded.Iterate(ctx, nil, pivot, func(e elements.Entry) (bool, error) {
			got[string(e.Key())] = e.(*mockEntry).val
			order = append(order, e.Key())
			return false, nil
		}); err != nil {
			t.Fatalf("Iterate on a freshly loaded pot returned an error (want: none): %v", err)
		}
		if len(got) != count {
			t.Fatalf("want %d entries, got %d", count, len(got))
		}
		for k, v := range want {
			if got[k] != v {
				t.Fatalf("entry %x: want %d, got %d", []byte(k), v, got[k])
			}
		}
		for i := 1; i < len(order); i++ {
			if bytes.Compare(order[i-1], order[i]) >= 0 {
				t.Fatalf("not in ascending order at %d: %x then %x", i, order[i-1], order[i])
			}
		}
	})

	t.Run("with a prefix, on the same loaded pot", func(t *testing.T) {
		// exercise a second Iterate on the same Index so we also cover forks
		// that a previous call may have already unpacked (Unpack must stay a
		// safe no-op on nodes that are now in memory).
		prefix := newDetMockEntry(t, 0).key[:1]
		n := 0
		if err := loaded.Iterate(ctx, prefix, make([]byte, 32), func(e elements.Entry) (bool, error) {
			if e.Key()[0] != prefix[0] {
				t.Fatalf("entry %x does not match prefix %x", e.Key(), prefix)
			}
			n++
			return false, nil
		}); err != nil {
			t.Fatalf("prefixed Iterate on a freshly loaded pot returned an error: %v", err)
		}
		if n == 0 {
			t.Fatalf("expected at least one entry under prefix %x", prefix)
		}
	})
}

func TestSize(t *testing.T) {
	count := 16
	test := func(t *testing.T, idx *pot.Index) {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		t.Run("add", func(t *testing.T) {
			for i := 0; i < count; i++ {
				size := idx.Size()
				if size != i {
					t.Fatalf("incorrect number of items. want %d, got %d", i, size)
				}
				idx.Add(ctx, newDetMockEntry(t, i))
			}
		})
		t.Run("update", func(t *testing.T) {
			for i := 0; i < count; i++ {
				idx.Add(ctx, &mockEntry{newDetMockEntry(t, i).key, 10000})
				size := idx.Size()
				if size != count {
					t.Fatalf("incorrect number of items. want %d, got %d", count, size)
				}
			}
		})
		t.Run("delete", func(t *testing.T) {
			for i := 0; i < count; i++ {
				idx.Delete(ctx, newDetMockEntry(t, i).key)
				size := idx.Size()
				if size != count-i-1 {
					t.Fatalf("incorrect number of items. want %d, got %d", count-i-1, size)
				}
			}
		})
	}
	t.Run("in memory", func(t *testing.T) {
		idx, err := pot.New(basePotMode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()
		test(t, idx)
	})
	t.Run("persisted", func(t *testing.T) {
		ls := persister.NewInmemLoadSaver()
		mode := elements.NewSwarmPot(basePotMode, ls, func(key []byte) elements.Entry { return &mockEntry{key: key} })
		idx, err := pot.New(mode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()
		test(t, idx)
	})
}

func TestPersistence(t *testing.T) {
	count := 200

	ls := persister.NewInmemLoadSaver()
	mode := elements.NewSwarmPot(basePotMode, ls, func(key []byte) elements.Entry { return &mockEntry{key: key} })
	idx, err := pot.New(mode)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for i := 0; i < count; i++ {
		idx.Add(ctx, newDetMockEntry(t, i))
	}
	idx.Close()

	ls = persister.NewInmemLoadSaver()
	mode = elements.NewSwarmPot(basePotMode, ls, func(key []byte) elements.Entry { return &mockEntry{key: key} })
	idx, err = pot.New(mode)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	for i := 0; i < count+10; i++ {
		idx.Add(ctx, newDetMockEntry(t, i))
	}
	for i := 0; i < count+10; i++ {
		checkFound(t, ctx, idx, newDetMockEntry(t, i))
	}
	t.Run("delete only existent tuple, then save", func(t *testing.T) {
		ls = persister.NewInmemLoadSaver()
		mode = elements.NewSwarmPot(basePotMode, ls, func(key []byte) elements.Entry { return &mockEntry{key: key} })
		idx, err = pot.New(mode)
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		e0 := newDetMockEntry(t, 0)
		idx.Add(ctx, e0)
		idx.Delete(ctx, e0.key)
		_, err := idx.Save(ctx)
		assert.Error(t, err)
	})
	t.Run("delete non-existent tuple from non empty POT, then save", func(t *testing.T) {
		ls = persister.NewInmemLoadSaver()
		mode = elements.NewSwarmPot(basePotMode, ls, func(key []byte) elements.Entry { return &mockEntry{key: key} })
		idx, err = pot.New(mode)
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		e0 := newDetMockEntry(t, 0)
		e1 := newDetMockEntry(t, 1)
		idx.Add(ctx, e0)
		idx.Delete(ctx, e1.key)
		_, err := idx.Save(ctx)
		assert.NoError(t, err)
	})
}

func TestConcurrency(t *testing.T) {
	test := func(t *testing.T, idx *pot.Index) {
		workers := 4
		count := 1000

		c := make(chan int, count)
		start := make(chan struct{})
		ctx := context.Background()
		eg, ectx := errgroup.WithContext(ctx)
		for k := 0; k < workers; k++ {
			k := k
			eg.Go(func() error {
				<-start
				for i := 0; i < count; i++ {
					j := i*workers + k
					e := newDetMockEntry(t, j)
					idx.Add(ctx, e)
					_, err := idx.Find(ctx, e.key)
					if err != nil {
						return err
					}
					select {
					case <-ectx.Done():
						return ectx.Err()
					case c <- j:
					}
				}
				return nil
			})
		}
		// parallel to these workers, other workers collect the inserted items and delete them
		for k := 0; k < workers-1; k++ {
			eg.Go(func() error {
				<-start
				for i := 0; i < count; i++ {
					var j int
					select {
					case j = <-c:
					case <-ectx.Done():
						return ectx.Err()
					}
					e := newDetMockEntry(t, j)
					idx.Delete(ctx, e.Key())
					_, err := idx.Find(ctx, e.key)
					if !errors.Is(err, elements.ErrNotFound) {
						return err
					}
				}
				return nil
			})
		}
		close(start)
		if err := eg.Wait(); err != nil {
			t.Fatal(err)
		}
		close(c)
		entered := make(map[int]struct{})
		for i := range c {
			_, err := idx.Find(ctx, newDetMockEntry(t, i).key)
			if err != nil {
				t.Fatalf("find %d: expected found. got %v", i, err)
			}
			entered[i] = struct{}{}
		}
		for i := 0; i < workers*count; i++ {
			if _, found := entered[i]; found {
				continue
			}
			_, err := idx.Find(ctx, newDetMockEntry(t, i).key)
			if !errors.Is(err, elements.ErrNotFound) {
				t.Fatalf("find %d: expected %v. got %v", i, elements.ErrNotFound, err)
			}
		}
	}

	t.Run("in memory", func(t *testing.T) {
		idx, err := pot.New(basePotMode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()
		test(t, idx)
	})
	t.Run("persisted", func(t *testing.T) {
		ls := persister.NewInmemLoadSaver()
		mode := elements.NewSwarmPot(basePotMode, ls, func(key []byte) elements.Entry { return &mockEntry{key: key} })
		idx, err := pot.New(mode)
		if err != nil {
			t.Fatal(err)
		}
		defer idx.Close()
		test(t, idx)
	})
}

func newDetMockEntry(t *testing.T, n int) *mockEntry {
	t.Helper()
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(n))
	hasher := sha256.New()
	if _, err := hasher.Write(buf); err != nil {
		t.Fatal(err)
	}
	return &mockEntry{hasher.Sum(nil), int(n)}
}

func checkFound(t *testing.T, ctx context.Context, idx *pot.Index, want *mockEntry) {
	t.Helper()
	e, err := idx.Find(ctx, want.Key())
	if err != nil {
		t.Fatal(err)
	}
	got, ok := e.(*mockEntry)
	if !ok {
		_ = e.(*mockEntry)
		t.Fatalf("incorrect value")
	}
	if !eq(want, got) {
		t.Fatalf("mismatch. want %v, got %v", want, got)
	}
}

func checkNotFound(t *testing.T, ctx context.Context, idx *pot.Index, want *mockEntry) {
	t.Helper()
	_, err := idx.Find(ctx, want.Key())
	if !errors.Is(err, elements.ErrNotFound) {
		t.Fatalf("incorrect error returned for %d. want %v, got %v", want.val, pot.ErrNotFound, err)
	}
}
