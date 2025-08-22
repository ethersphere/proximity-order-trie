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
				if err := idx.Iterate(s, pivot, func(e elements.Entry) (bool, error) {
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
			if err := idx.Iterate(nil, pivot, func(e elements.Entry) (bool, error) {
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
