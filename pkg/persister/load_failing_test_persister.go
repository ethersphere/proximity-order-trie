package persister

import (
	"context"
	"fmt"
)

type LoadFailingTestLoadSaver struct {
	store map[[32]byte][]byte
}

func NewLoadFailingTestLoadSaver() *LoadFailingTestLoadSaver {
	return &LoadFailingTestLoadSaver{
		store: make(map[[32]byte][]byte),
	}
}

func (ls *LoadFailingTestLoadSaver) Load(ctx context.Context, reference []byte) ([]byte, error) {

	return nil, fmt.Errorf("mock test error in LoadFailingTestLoadSave.Load()")
}

func (ls *LoadFailingTestLoadSaver) Save(ctx context.Context, data []byte) ([]byte, error) {
	ref := getBMTHash(data)
	ls.store[ref] = data
	return ref[:], nil
}

