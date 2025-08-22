package proof_test

import (
	"context"
	"testing"

	pot "github.com/ethersphere/proximity-order-trie"
	"github.com/ethersphere/proximity-order-trie/pkg/elements"
	"github.com/ethersphere/proximity-order-trie/pkg/persister"
	"github.com/ethersphere/proximity-order-trie/pkg/proof"
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
			nodeData: createValidNodeData(t, 0, nil),
			wantErr:  false,
		},
		{
			name:     "valid node data with 3 children",
			nodeData: createValidNodeData(t, 3, nil),
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
		nodeData: createValidNodeData(t, 10, nil), // Create a node with 10 children
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proofs, err := proof.CreateEntryProof(tt.nodeData)
			// Validate that the proofs are properly created
			prover := proof.NewBMTProver()
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
			if proofs == nil {
				t.Fatal("CreateEntryProof() returned nil proof for valid data")
			}

			err = proof.ValidateEntryProof(nodeHash, proofs)
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
func createValidNodeData(t *testing.T, childCount int, ls persister.LoadSaver) []byte {
	t.Helper()
	node := elements.NewSwarmNode(func(key []byte) elements.Entry { e, _ := pot.NewSwarmEntry(key, make([]byte, 0)); return e })
	parentKey := make([]byte, 32) // parent key is all zeroes as defined at line 108
	entry, _ := pot.NewSwarmEntry(parentKey, []byte{55})
	node.Pin(entry)

	// Add the specified number of child nodes
	for i := 0; i < childCount; i++ {
		childKey := make([]byte, 32)
		childKey[0] = byte(i)
		childNode := elements.NewSwarmNode(func(key []byte) elements.Entry { e, _ := pot.NewSwarmEntry(key, make([]byte, 0)); return e })
		childEntry, _ := pot.NewSwarmEntry(childKey, []byte{byte(i)})
		childNode.Pin(childEntry)

		// set reference to child node
		if ls != nil {
			childData, err := childNode.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			ref, err := ls.Save(context.Background(), childData)
			if err != nil {
				t.Fatal(err)
			}
			childNode.SetReference(ref)
		} else {
			childNode.SetReference(childKey)
		}

		po := elements.PO(parentKey, childKey, 0)

		node.Append(elements.CNode{
			At:   po,
			Node: childNode,
		})
	}

	data, err := node.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	return data
}
