package proof

import (
	"testing"

	pot "github.com/ethersphere/proximity-order-trie"
	"github.com/ethersphere/proximity-order-trie/pkg/elements"
)

func TestCreateEntryProof(t *testing.T) {
	tests := []struct {
		name          string
		nodeData      []byte
		wantErr       bool
		errorContains string
	}{
		{
			name:          "empty node data",
			nodeData:      []byte{},
			wantErr:       true,
			errorContains: "empty node data",
		},
		{
			name:          "zero entry size",
			nodeData:      createNodeWithZeroEntrySize(),
			wantErr:       true,
			errorContains: "entry offset is out of bounds",
		},
		{
			name:     "valid node data without children",
			nodeData: createValidNodeData(0),
			wantErr:  false,
		},
		{
			name:     "valid node data with 3 children",
			nodeData: createValidNodeData(3),
			wantErr:  false,
		},
	}

	// Test with different numbers of child nodes
	tests = append(tests, struct {
		name          string
		nodeData      []byte
		wantErr       bool
		errorContains string
	}{
		name:     "valid node data with many children",
		nodeData: createValidNodeData(10), // Create a node with 10 children
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proof, err := CreateEntryProof(tt.nodeData)
			// Validate that the proofs are properly created
			prover := NewBMTProver()
			prover.SetHeaderInt64(int64(len(tt.nodeData)))
			_, _ = prover.Write(tt.nodeData)
			nodeHash := prover.Sum(nil)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("CreateEntryProof() expected error containing %q, got nil", tt.errorContains)
					return
				}
				if tt.errorContains != "" && err.Error() != tt.errorContains {
					t.Errorf("CreateEntryProof() error = %v, want error containing %v", err, tt.errorContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("CreateEntryProof() unexpected error: %v", err)
			}

			// For valid cases, validate the returned proof structure
			if proof == nil {
				t.Fatal("CreateEntryProof() returned nil proof for valid data")
			}

			err = ValidateEntryProof(nodeHash, proof)
			if err != nil {
				t.Errorf("ValidateEntryProof() unexpected error: %v", err)
			}
		})
	}
}

// createNodeWithZeroEntrySize creates a node with a structure that will result in
// a entry offset is out of bounds error case
func createNodeWithZeroEntrySize() []byte {
	// Create a node with enough data to pass length check but with 0 entry size
	data := make([]byte, 128)
	for i := 0; i < 32; i++ {
		data[i] = byte(i)
	}

	data[32] = 0x80 // Only the highest bit is set - indicating a key at pos 0

	return data // Entry size will be 0 at the calculated offset
}

// createValidNodeData creates valid node data with an entry and the specified number of child nodes
func createValidNodeData(childCount int) []byte {
	node := elements.NewSwarmNode(func(key []byte) elements.Entry { e, _ := pot.NewSwarmEntry(key, make([]byte, 0)); return e })
	entry, _ := pot.NewSwarmEntry(make([]byte, 32), []byte{55})
	node.Pin(entry)

	// Add the specified number of child nodes
	for i := 0; i < childCount; i++ {
		childRef := make([]byte, 32)
		childRef[0] = byte(i)
		childNode := elements.NewSwarmNode(func(key []byte) elements.Entry { e, _ := pot.NewSwarmEntry(key, make([]byte, 0)); return e })
		childEntry, _ := pot.NewSwarmEntry(childRef, []byte{byte(i)})
		childNode.Pin(childEntry)
		childNode.SetReference(childRef)

		node.Append(elements.CNode{
			At:   i,
			Node: childNode,
		})
	}

	data, err := node.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return data
}
