package persister

import (
	"context"
	"fmt"
)

type SaveFailingTestLoadSaver struct {
	store map[[32]byte][]byte
}

func NewSaveFailingTestLoadSaver() *SaveFailingTestLoadSaver {
	return &SaveFailingTestLoadSaver{
		store: make(map[[32]byte][]byte),
	}
}

func (ls *SaveFailingTestLoadSaver) Load(ctx context.Context, reference []byte) ([]byte, error) {
	if len(reference) != 32 {
		return nil, fmt.Errorf("reference must be 32 bytes, got %d", len(reference))
	}
	var refArr [32]byte
	copy(refArr[:], reference)
	data, ok := ls.store[refArr]
	if !ok {
		return nil, fmt.Errorf("reference not found")
	}
	return data, nil
}

func (ls *SaveFailingTestLoadSaver) Save(ctx context.Context, data []byte) ([]byte, error) {

	return nil, fmt.Errorf("mock test error in SaveFailingTestLoadSave.Save()")
}
