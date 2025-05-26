package pot_test

import (
	"context"
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
	t.Run("Save KVS with one item, no error, pre-save value exist", func(t *testing.T) {
		s1, _ := pot.NewSwarmKvs(ls)

		err := s1.Put(ctx, key1, val1)
		assert.NoError(t, err)

		ref, err := s1.Save(ctx)
		assert.NoError(t, err)

		s2, err := pot.NewSwarmKvsReference(ls, ref)
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
		kvs2, err := pot.NewSwarmKvsReference(ls, ref)
		assert.NoError(t, err)
		err = kvs2.Put(ctx, key2, val2)
		assert.NoError(t, err)

		val, err := kvs2.Get(ctx, key2)
		assert.NoError(t, err)
		assert.Equal(t, val2, val)
	})
}
