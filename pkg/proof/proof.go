package proof

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ethersphere/bee/v2/pkg/bmt"
	"github.com/ethersphere/proximity-order-trie/pkg/elements"
	"golang.org/x/crypto/sha3"
)

// BMTProver handles inclusion proofs for entries in the proximity-order-trie
type BMTProver struct {
	*bmt.Prover
}

// NewBMTProver creates a new BMT prover instance
func NewBMTProver() *BMTProver {
	hasherFunc := sha3.NewLegacyKeccak256
	pool := bmt.NewPool(bmt.NewConf(hasherFunc, 128, 0))

	return &BMTProver{
		Prover: &bmt.Prover{Hasher: pool.Get()},
	}
}

// ForkNodeProof contains the proof data for a fork node
type ForkNodeProof struct {
	// BitVectorProof contains the proof for the bitvector representing the fork structure
	BitVectorProof *bmt.Proof
	// ForkReferenceProof contains the proof for the specific fork reference
	ForkReferenceProof *bmt.Proof
	// ForkPO is the proximity order of the specific fork
	ForkPO int
}

// CreateForkNodeProof generates a proof for a fork node, including both the forkmap bitvector proof
// and the proof for the specific fork reference that matches the target key.
func CreateForkNodeProof(parentData, targetKey []byte) (*ForkNodeProof, error) {
	if len(parentData) == 0 {
		return nil, fmt.Errorf("empty parent node")
	}

	if len(targetKey) == 0 {
		return nil, fmt.Errorf("empty target key")
	}

	parentKey := parentData[:32]
	if bytes.Equal(parentKey, targetKey) {
		return nil, fmt.Errorf("parent key and target key are the same")
	}

	bitVector := parentData[32:64]

	// Find the common prefix length between the node key and target key
	// This tells us which fork is relevant for our target key
	var forkPO int
	found := false
	for i := 0; i < elements.MaxDepth; i++ {
		bytePos := i / 8
		bitPos := i % 8

		mask := byte(1 << (7 - bitPos))
		if (parentKey[bytePos] & mask) != (targetKey[bytePos] & mask) {
			// Found the first bit that differs - this is the PO we need
			forkPO = i
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("no matching fork found for the target key at node %s", parentKey)
	}
	// check specificForkPO is in the bitvector
	if bitVector[forkPO/8]&(1<<(7-forkPO%8)) == 0 {
		return nil, fmt.Errorf("specific fork PO %d is not in the bitvector", forkPO)
	}
	// count how many forks are before the specificForkPO
	forkCount := 1
	for i := 0; i < forkPO; i++ {
		if bitVector[i/8]&(1<<(7-i%8)) != 0 {
			forkCount++
		}
	}

	prover := NewBMTProver()
	// Use the bitvector as the data to prove
	prover.SetHeaderInt64(int64(len(parentData)))
	prover.Write(parentData)

	bitVectorProof := prover.Proof(1)
	forkReferenceProof := prover.Proof(forkCount + 2) // +2 because of the key and the bitvector

	return &ForkNodeProof{
		BitVectorProof:     &bitVectorProof,
		ForkReferenceProof: &forkReferenceProof,
		ForkPO:             forkPO,
	}, nil
}

// EntryProof contains the proof data for an entry value
type EntryProof struct {
	// Proof is the BMT proof for the entry value
	Proof *bmt.Proof
	// Size is the size of the entry value in bytes
	Size uint32
	// NodeKey is the key of the node containing the entry
	NodeKey []byte
}

// CreateEntryProof generates a proof for an entry value within a node
// The nodeData should be the binary representation of the node containing the entry
func CreateEntryProof(nodeData []byte) (*EntryProof, error) {
	if len(nodeData) == 0 {
		return nil, fmt.Errorf("empty node data")
	}

	// Extract the node key (first 32 bytes)
	nodeKey := nodeData[:32]

	bitMap := nodeData[32:64]
	oneCount := 0
	for i := 0; i < elements.MaxDepth; i++ {
		if bitMap[i/8]&(1<<(7-i%8)) == 1 {
			oneCount++
		}
	}

	// TODO: padding after descendantCounts
	entryOffset := 64 + oneCount*32 + oneCount*4

	// Extract entry size - this is typically stored as a 4-byte uint32 before the entry data
	// The exact location depends on the node structure and how the entry is stored
	entrySize := binary.BigEndian.Uint32(nodeData[entryOffset:])
	if entrySize == 0 {
		return nil, fmt.Errorf("entry size is 0")
	}

	segmentIndex := entryOffset / 32

	prover := NewBMTProver()
	prover.SetHeaderInt64(int64(len(nodeData)))
	prover.Write(nodeData)

	entryProof := prover.Proof(segmentIndex)

	return &EntryProof{
		Proof:   &entryProof,
		Size:    entrySize,
		NodeKey: nodeKey,
	}, nil
}
