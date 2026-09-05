package pot_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"math/rand"
	"testing"

	pot "github.com/ethersphere/proximity-order-trie"
	"github.com/ethersphere/proximity-order-trie/pkg/persister"
	"github.com/stretchr/testify/assert"
)

func createLs() persister.LoadSaver {
	return persister.NewInmemLoadSaver()
}

func keyValuePair(t *testing.T) ([]byte, []byte) {
	t.Helper()

	key := make([]byte, 32)
	value := make([]byte, rand.Intn(79)+22)
	_, err := rand.Read(key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = rand.Read(value)
	if err != nil {
		t.Fatal(err)
	}
	return key, value
}

func TestPotKvs_Save(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ls := createLs()
	key1, val1 := keyValuePair(t)
	key2, val2 := keyValuePair(t)
	t.Run("Save empty KVS return error", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		ref, err := s.Save(ctx)
		assert.Error(t, err)
		assert.Nil(t, ref)
	})
	t.Run("Save not empty KVS return valid swarm address", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		err := s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		ref, err := s.Save(ctx)
		assert.NoError(t, err)
		assert.True(t, len(ref) > 0)
	})
 	t.Run("Put key-value pair a second time, then save", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		err := s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		err = s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
	})
 	t.Run("Save twice", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		err := s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
	})
	t.Run("Change key's value between two saves", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		err := s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
		err = s.Put(ctx, key1, val2)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
	})
	t.Run("Store same value under different key between two saves", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		err := s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
		err = s.Put(ctx, key2, val1)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
	})
	t.Run("Put same key-value pair a second time between saves", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		err := s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
		err = s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		_, err = s.Save(ctx)
		assert.NoError(t, err)
	})
	t.Run("Put same key-value pair a second time, on new-by-reference KVS, between saves", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(ls)
		err := s.Put(ctx, key1, val1)
		assert.NoError(t, err)
		ref, err := s.Save(ctx)
		assert.NoError(t, err)
		s2, err := pot.NewSwarmKvsReference(ctx, ls, ref)
		assert.NoError(t, err)
		err = s2.Put(ctx, key1, val1)
		assert.NoError(t, err)
		_, err = s2.Save(ctx)
		assert.NoError(t, err)
	})
	t.Run("Save KVS with one item, no error, pre-save value exist", func(t *testing.T) {
		s1, _ := pot.NewSwarmKvs(ls)

		err := s1.Put(ctx, key1, val1)
		assert.NoError(t, err)

		ref, err := s1.Save(ctx)
		assert.NoError(t, err)

		s2, err := pot.NewSwarmKvsReference(ctx, ls, ref)
		assert.NoError(t, err)

		val, err := s2.Get(ctx, key1)
		assert.NoError(t, err)
		assert.Equal(t, val1, val)
	})
	t.Run("Save KVS and add one item, no error, after-save value exist", func(t *testing.T) {
		ls := createLs()
		kvs1, _ := pot.NewSwarmKvs(ls)

		err := kvs1.Put(ctx, key1, val1)
		assert.NoError(t, err)
		ref, err := kvs1.Save(ctx)
		assert.NoError(t, err)

		// New KVS
		kvs2, err := pot.NewSwarmKvsReference(ctx, ls, ref)
		assert.NoError(t, err)
		err = kvs2.Put(ctx, key2, val2)
		assert.NoError(t, err)

		val, err := kvs2.Get(ctx, key2)
		assert.NoError(t, err)
		assert.Equal(t, val2, val)
	})
	t.Run("Save KVS and delete one item, test that it is deleted, after-save value exist", func(t *testing.T) {
		ls := createLs()
		kvs1, _ := pot.NewSwarmKvs(ls)

		err := kvs1.Put(ctx, key1, val1)
		assert.NoError(t, err)
		val, err := kvs1.Get(ctx, key1)
		assert.NoError(t, err)
		assert.Equal(t, val1, val)
		ref, err := kvs1.Save(ctx)
		assert.NoError(t, err)
		err = kvs1.Delete(ctx, key1)
		assert.NoError(t, err)
		val, err = kvs1.Get(ctx, key1)
		assert.Error(t, err, "not found")

		// New KVS
		kvs2, err := pot.NewSwarmKvsReference(ctx, ls, ref)
		assert.NoError(t, err)

		val, err = kvs2.Get(ctx, key1)
		assert.NoError(t, err)
		assert.Equal(t, val1, val)
	})
	t.Run("Save KVS with two items, after-load values exist", func(t *testing.T) {
		ls := createLs()
		kvs1, _ := pot.NewSwarmKvs(ls)

		err := kvs1.Put(ctx, key1, val1)
		assert.NoError(t, err)

		err = kvs1.Put(ctx, key2, val2)
		assert.NoError(t, err)

		ref, err := kvs1.Save(ctx)
		assert.NoError(t, err)

		// New KVS
		kvs2, err := pot.NewSwarmKvsReference(ctx, ls, ref)
		assert.NoError(t, err)

		val, err := kvs2.Get(ctx, key1)
		assert.NoError(t, err)
		assert.Equal(t, val1, val)

		val, err = kvs2.Get(ctx, key2)
		assert.NoError(t, err)
		assert.Equal(t, val2, val)
	})
	t.Run("Create KVS, write to it, close it", func(t *testing.T) {
		ls := createLs()
		kvs1, _ := pot.NewSwarmKvs(ls)

		err := kvs1.Put(ctx, key1, val1)
		assert.NoError(t, err)

		_, err = kvs1.Save(ctx)
		assert.NoError(t, err)

		err = kvs1.Close()
		assert.NoError(t, err)
       })
}

// sha256Key derives a deterministic, pseudo-random-looking 32-byte key from
// n, the same way index_test.go's newDetMockEntry does. Sequential or purely
// random (via keyValuePair, used elsewhere in this file) keys are both poor
// at this: sequential keys rarely fork at bit 0, so they wouldn't have caught
// the off-by-one this suite guards against, and unseeded random keys make a
// failure hard to reproduce. A deterministic hash gives repeatable tests that
// still spread realistically across the key space.
func sha256Key(n int) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(n))
	h := sha256.Sum256(buf)
	return h[:]
}

func TestSwarmKvs_Iterate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("empty KVS: Size is 0, Iterate visits nothing", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(createLs())
		assert.Equal(t, 0, s.Size())
		n := 0
		err := s.Iterate(ctx, nil, make([]byte, 32), func(key, value []byte) (bool, error) {
			n++
			return false, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("Size and Iterate agree on count, in memory", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(createLs())
		count := 300
		want := make(map[string][]byte, count)
		for i := 0; i < count; i++ {
			k := sha256Key(i)
			v := []byte{byte(i), byte(i >> 8)}
			assert.NoError(t, s.Put(ctx, k, v))
			want[string(k)] = v
		}
		assert.Equal(t, count, s.Size())

		got := make(map[string][]byte, count)
		var order [][]byte
		err := s.Iterate(ctx, nil, make([]byte, 32), func(key, value []byte) (bool, error) {
			got[string(key)] = value
			order = append(order, key)
			return false, nil
		})
		assert.NoError(t, err)
		assert.Equal(t, len(want), len(got))
		for k, v := range want {
			assert.Equal(t, v, got[k])
		}
		for i := 1; i < len(order); i++ {
			assert.True(t, bytes.Compare(order[i-1], order[i]) < 0, "not in ascending order at %d", i)
		}
	})

	t.Run("Iterate survives a save + load round trip and honours a prefix", func(t *testing.T) {
		// This is the scenario the two bugs this fork fixes both needed:
		// a *fresh* KVS opened from a save reference, not the same
		// instance that wrote the data, where every node starts out
		// packed. See PR description / index_test.go for the two bugs.
		ls := createLs()
		s, _ := pot.NewSwarmKvs(ls)
		count := 300
		for i := 0; i < count; i++ {
			assert.NoError(t, s.Put(ctx, sha256Key(i), []byte{byte(i)}))
		}
		ref, err := s.Save(ctx)
		assert.NoError(t, err)
		assert.NoError(t, s.Close())

		loaded, err := pot.NewSwarmKvsReference(ctx, ls, ref)
		assert.NoError(t, err)
		defer loaded.Close()

		assert.Equal(t, count, loaded.Size())
		n := 0
		assert.NoError(t, loaded.Iterate(ctx, nil, make([]byte, 32), func(key, value []byte) (bool, error) {
			n++
			return false, nil
		}))
		assert.Equal(t, count, n, "a loaded KVS must see every entry, not silently drop some")

		// pick a one-byte prefix several keys share and check Iterate only
		// returns matches, and Size-via-Iterate-count agrees
		byPrefix := map[byte]int{}
		for i := 0; i < count; i++ {
			byPrefix[sha256Key(i)[0]]++
		}
		var bestPrefix byte
		bestN := 0
		for p, c := range byPrefix {
			if c > bestN {
				bestPrefix, bestN = p, c
			}
		}
		matched := 0
		assert.NoError(t, loaded.Iterate(ctx, []byte{bestPrefix}, make([]byte, 32), func(key, value []byte) (bool, error) {
			assert.Equal(t, bestPrefix, key[0])
			matched++
			return false, nil
		}))
		assert.Equal(t, bestN, matched)
	})

	t.Run("Iterate can stop early", func(t *testing.T) {
		s, _ := pot.NewSwarmKvs(createLs())
		for i := 0; i < 50; i++ {
			assert.NoError(t, s.Put(ctx, sha256Key(i), []byte{byte(i)}))
		}
		n := 0
		err := s.Iterate(ctx, nil, make([]byte, 32), func(key, value []byte) (bool, error) {
			n++
			return n == 5, nil // stop after the 5th
		})
		assert.NoError(t, err)
		assert.Equal(t, 5, n)
	})
}
