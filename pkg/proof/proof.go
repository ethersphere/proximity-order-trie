package proof

import (
	"bytes"
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
	pool := bmt.NewPool(bmt.NewConf(hasherFunc, 128, 1))

	return &BMTProver{
		Prover: &bmt.Prover{Hasher: pool.Get()},
	}
}

// ForkRefProof contains the proof data for a fork node
type ForkRefProof struct {
	// BitVectorProof contains the proof for the bitvector representing the fork structure
	BitVectorProof *bmt.Proof
	// ForkReferenceProof contains the proof for the specific fork reference
	ForkReferenceProof *bmt.Proof
	// ForkPO is the proximity order of the specific fork
	ForkPO int
}

// CreateForkNodeProof generates a proof for a fork node, including both the forkmap bitvector proof
// and the proof for the specific fork reference that matches the target key.
func CreateForkNodeProof(parentData, targetKey []byte) (*ForkRefProof, error) {
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
	forkCount := 0
	for i := 0; i < forkPO; i++ {
		if bitVector[i/8]&(1<<(7-i%8)) != 0 {
			forkCount++
		}
	}

	prover := NewBMTProver()
	// Use the bitvector as the data to prove
	prover.SetHeaderInt64(int64(len(parentData)))
	prover.Write(parentData)
	_, _ = prover.Hash(nil) // necessary to fill up bmt

	bitVectorProof := prover.Proof(1)
	forkReferenceProof := prover.Proof(forkCount + 2) // +2 because of the key and the bitvector

	return &ForkRefProof{
		BitVectorProof:     &bitVectorProof,
		ForkReferenceProof: &forkReferenceProof,
		ForkPO:             forkPO,
	}, nil
}

// EntryProof contains the proof data for an entry value
type EntryProof struct {
	// EntryProof is the BMT proof for the entry value
	EntryProof *bmt.Proof
	// BitVectorProof is the BMT proof for the bitvector and the node's key
	BitVectorProof *bmt.Proof
}

// CreateEntryProof generates a proof for an entry value within a node
// The nodeData should be the binary representation of the node containing the entry
func CreateEntryProof(nodeData []byte) (*EntryProof, error) {
	// TODO: proof for more than one segment entry
	dl := len(nodeData)
	if dl == 0 {
		return nil, fmt.Errorf("empty node data")
	}

	bitMap := nodeData[32:64]
	oneCount := 0
	for i := 0; i < elements.MaxDepth; i++ {
		if (bitMap[i/8]>>(7-i%8))&1 == 1 {
			oneCount++
		}
	}

	takenBytes := (oneCount * 4) % 32
	paddingBytes := 0
	if takenBytes > 0 {
		paddingBytes = 32 - takenBytes
	}
	entryOffset := 64 + oneCount*32 + oneCount*4 + paddingBytes

	if entryOffset >= dl {
		return nil, fmt.Errorf("entry offset is out of bounds")
	}

	entrySegmentIndex := entryOffset / 32

	prover := NewBMTProver()
	prover.SetHeaderInt64(int64(len(nodeData)))
	prover.Write(nodeData)
	_ = prover.Sum(nil) // necessary to fill up bmt field of prover
	// prove bitMap for calculating entrySegementIndex
	// along with the element's full key
	bitVectorProof := prover.Proof(1)
	entryProof := prover.Proof(entrySegmentIndex)

	return &EntryProof{
		EntryProof:     &entryProof,
		BitVectorProof: &bitVectorProof,
	}, nil
}

// ValidateEntryProof validates an entry proof against a given Swarm hash
// Returns nil if the proof is valid, otherwise returns an error
func ValidateEntryProof(nodeHash []byte, proof *EntryProof) error {
	if proof == nil {
		return fmt.Errorf("nil entry proof")
	}

	if len(nodeHash) != 32 {
		return fmt.Errorf("invalid node hash length: %d, expected 32", len(nodeHash))
	}

	hashcalc1, err := Verify(*proof.BitVectorProof)
	if err != nil {
		return fmt.Errorf("bitvector proof verification failed: %w", err)
	}
	hashcalc2, err := Verify(*proof.EntryProof)
	if err != nil {
		return fmt.Errorf("entry proof verification failed: %w", err)
	}

	if !bytes.Equal(hashcalc1, hashcalc2) {
		return fmt.Errorf("BitVectorProof and EntryProof do not match")
	}
	if !bytes.Equal(hashcalc1, nodeHash) {
		return fmt.Errorf("calculated proof hashes and the nodeHash do not match")
	}

	return nil
}
