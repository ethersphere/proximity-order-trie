package proof_test

import (
	"context"
	"testing"

	pot "github.com/ethersphere/proximity-order-trie"
	"github.com/ethersphere/proximity-order-trie/pkg/elements"
	"github.com/ethersphere/proximity-order-trie/pkg/persister"
	"github.com/ethersphere/proximity-order-trie/pkg/proof"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateForkPathProof tests the fork path proof creation functionality
func TestForkPathProof(t *testing.T) {
	// Create an in-memory persister
	ls := persister.NewInmemLoadSaver()

	tests := []struct {
		name          string
		levels        int // Number of levels in hierarchy (0=root only, 1=root+children, 2=root+children+grandchildren)
		wantErr       bool
		errorContains string
	}{
		{
			name:    "on level proving - search for root",
			levels:  0,
			wantErr: false,
		},
		{
			name:    "one level below proving",
			levels:  1,
			wantErr: false,
		},
		{
			name:    "two levels below proving",
			levels:  2,
			wantErr: false,
		},
		{
			name:          "nil root node",
			levels:        0,
			wantErr:       true,
			errorContains: "root node is nil",
		},
		{
			name:          "nil load saver",
			levels:        1,
			wantErr:       true,
			errorContains: "load saver is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle special test cases
			if tt.name == "nil root node" {
				_, err := proof.CreateForkPathProof(nil, ls, make([]byte, 32))
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}
			if tt.name == "nil load saver" {
				root, _ := createTestTrie(t, ls, 1)
				_, err := proof.CreateForkPathProof(root, nil, make([]byte, 32))
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}

			root, keys := createTestTrie(t, ls, tt.levels)

			var targetKey []byte
			switch tt.levels {
			case 0:
				targetKey = keys[0]
			case 1:
				targetKey = keys[1]
			case 2:
				targetKey = keys[2]
			}

			proofs, err := proof.CreateForkPathProof(root, ls, targetKey)
			if err != nil {
				t.Errorf("CreateForkPathProof() unexpected error: %v", err)
				return
			}

			jsonProofsData := proofs.JSON()
			t.Logf("Proofs: %s", jsonProofsData)

			// print hex value of proofs.RootReference
			t.Logf("RootReference: %x", proofs.RootReference)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				return
			}

			// Validate successful proof
			require.NoError(t, err)
			require.NotNil(t, proofs)
			assert.Equal(t, targetKey, proofs.TargetKey)

			// Validate proof structure based on level
			assert.NotNil(t, proofs.RootReference)
			switch tt.levels {
			case 0:
				// Root level proof should have entry proof but no fork proofs
				assert.NotNil(t, proofs.EntryProof)
				assert.Len(t, proofs.ForkRefProofs, 0)
			case 1:
				// One level below should have 1 fork proof
				assert.NotNil(t, proofs.EntryProof)
				assert.Len(t, proofs.ForkRefProofs, 1)

				hashcalc1, err := proof.Verify(*proofs.ForkRefProofs[0].BitVectorProof)
				if err != nil {
					t.Errorf("Verify() unexpected error: %v", err)
				}
				hashcalc2, err := proof.Verify(*proofs.ForkRefProofs[0].ForkReferenceProof)
				if err != nil {
					t.Errorf("Verify() unexpected error: %v", err)
				}
				assert.Equal(t, proofs.RootReference, hashcalc1)
				assert.Equal(t, proofs.RootReference, hashcalc2)
			case 2:
				// Two levels below should have 2 fork proofs
				assert.NotNil(t, proofs.EntryProof)
				assert.Len(t, proofs.ForkRefProofs, 2)
				hashcalc11, err := proof.Verify(*proofs.ForkRefProofs[0].BitVectorProof)
				if err != nil {
					t.Errorf("Verify() unexpected error: %v", err)
				}
				hashcalc12, err := proof.Verify(*proofs.ForkRefProofs[0].ForkReferenceProof)
				if err != nil {
					t.Errorf("Verify() unexpected error: %v", err)
				}
				assert.Equal(t, proofs.RootReference, hashcalc11)
				assert.Equal(t, proofs.RootReference, hashcalc12)

				hashcalc21, err := proof.Verify(*proofs.ForkRefProofs[1].BitVectorProof)
				if err != nil {
					t.Errorf("Verify() unexpected error: %v", err)
				}
				hashcalc22, err := proof.Verify(*proofs.ForkRefProofs[1].ForkReferenceProof)
				if err != nil {
					t.Errorf("Verify() unexpected error: %v", err)
				}
				assert.NotEqual(t, hashcalc11, hashcalc21)
				assert.Equal(t, proofs.ForkRefProofs[0].ForkReferenceProof.ProveSegment, hashcalc21)
				assert.Equal(t, proofs.ForkRefProofs[0].ForkReferenceProof.ProveSegment, hashcalc22)
			}
		})
	}
}

// createTestTrie creates a test trie with a specified number of levels
// Returns the root node, and a slice of keys for testing
func createTestTrie(t *testing.T, ls persister.LoadSaver, levels int) (elements.Node, [][]byte) {
	// Create root node with all zeros key
	rootKey := make([]byte, 32)
	rootNode := elements.NewSwarmNode(func(key []byte) elements.Entry {
		e, _ := pot.NewSwarmEntry(key, make([]byte, 0))
		return e
	})
	rootEntry, _ := pot.NewSwarmEntry(rootKey, []byte{0})
	rootNode.Pin(rootEntry)

	// We'll store one target key per level
	keys := make([][]byte, 0, levels+1)
	keys = append(keys, rootKey)

	if levels >= 1 {
		// Create a level 1 key with a specific bit pattern to ensure known PO
		level1Key := make([]byte, 32)
		level1Key[0] = 0x80 // Set the highest bit in the first byte
		keys = append(keys, level1Key)

		level1Node := elements.NewSwarmNode(func(key []byte) elements.Entry {
			e, _ := pot.NewSwarmEntry(key, make([]byte, 0))
			return e
		})
		level1Entry, _ := pot.NewSwarmEntry(level1Key, []byte{level1Key[0]})
		level1Node.Pin(level1Entry)

		if levels >= 2 {
			level2Key := make([]byte, 32)
			level2Key[0] = 0x80 // Keep the same bit in first byte
			level2Key[1] = 0x80 // Set highest bit in second byte
			keys = append(keys, level2Key)

			level2Node := elements.NewSwarmNode(func(key []byte) elements.Entry {
				e, _ := pot.NewSwarmEntry(key, make([]byte, 0))
				return e
			})
			level2Entry, _ := pot.NewSwarmEntry(level2Key, []byte{level2Key[0], level2Key[1]})
			level2Node.Pin(level2Entry)

			// Persist level 2 node
			level2Data, err := level2Node.MarshalBinary()
			require.NoError(t, err)
			level2Ref, err := ls.Save(context.Background(), level2Data)
			require.NoError(t, err)
			level2Node.SetReference(level2Ref)

			// Calculate PO between level1Key and level2Key (will be 8)
			po := elements.PO(level1Key, level2Key, 0)
			level1Node.Append(elements.CNode{
				At:   po,
				Node: level2Node,
			})
		}

		// Persist level 1 node
		level1Data, err := level1Node.MarshalBinary()
		require.NoError(t, err)
		level1Ref, err := ls.Save(context.Background(), level1Data)
		require.NoError(t, err)
		level1Node.SetReference(level1Ref)

		// Calculate PO between rootKey and level1Key (will be 0)
		po := elements.PO(rootKey, level1Key, 0)
		rootNode.Append(elements.CNode{
			At:   po,
			Node: level1Node,
		})
	}

	// Persist root node
	rootData, err := rootNode.MarshalBinary()
	require.NoError(t, err)
	rootRef, err := ls.Save(context.Background(), rootData)
	require.NoError(t, err)
	rootNode.SetReference(rootRef)

	return rootNode, keys
}
