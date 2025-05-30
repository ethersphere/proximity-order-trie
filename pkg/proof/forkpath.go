package proof

import (
	"context"
	"fmt"

	"github.com/ethersphere/proximity-order-trie/pkg/elements"
	"github.com/ethersphere/proximity-order-trie/pkg/persister"
)

// ForkPathProof represents a path of proofs from a root node to a target node
type ForkPathProof struct {
	// ForkRefProofs contains all the fork node proofs in the path
	ForkRefProofs []*ForkRefProof
	// RootReference is the reference to the root node
	RootReference []byte
	// TargetKey is the key we were looking for
	TargetKey []byte
	// EntryProof contains the value proof for the target key
	EntryProof *EntryProof
}

// CreateForkPathProof generates a path of proofs from the root node to the target key.
// It iteratively loads nodes and creates proofs until it reaches the target key or encounters an error.
func CreateForkPathProof(rootNode elements.Node, ls persister.LoadSaver, targetKey []byte) (*ForkPathProof, error) {
	if rootNode == nil {
		return nil, fmt.Errorf("root node is nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("load saver is nil")
	}
	if len(targetKey) == 0 {
		return nil, fmt.Errorf("target key is empty")
	}

	// Get the Swarm reference from the root node
	swarmNode, ok := rootNode.(*elements.SwarmNode)
	if !ok {
		return nil, fmt.Errorf("root node is not a SwarmNode")
	}

	rootRef := swarmNode.Reference()
	if len(rootRef) == 0 {
		return nil, fmt.Errorf("root node has no reference")
	}

	// Initialize the path
	path := &ForkPathProof{
		ForkRefProofs: make([]*ForkRefProof, 0),
		RootReference: rootRef,
		TargetKey:     targetKey,
	}

	// Load the initial node data
	ctx := context.Background()
	currentNodeData, err := swarmNode.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("failed to load root node: %w", err)
	}

	// Iteratively create proofs and load nodes
	for {
		// Create a proof for the current node
		proof, err := CreateForkNodeProof(currentNodeData, targetKey)
		if err != nil {
			// If we've reached the target key, we're done
			if err.Error() == "parent key and target key are the same" {
				// Save the final node data
				entryProof, err := CreateEntryProof(currentNodeData)
				if err != nil {
					return nil, fmt.Errorf("failed to create entry proof: %w", err)
				}
				path.EntryProof = entryProof
				break
			}
			return nil, fmt.Errorf("failed to create fork node proof: %w", err)
		}
		path.ForkRefProofs = append(path.ForkRefProofs, proof)

		forkRef := proof.ForkReferenceProof.ProveSegment

		nextNodeData, err := ls.Load(ctx, forkRef)
		if err != nil {
			return nil, fmt.Errorf("failed to load next node with reference %x: %w", forkRef, err)
		}
		currentNodeData = nextNodeData
	}

	return path, nil
}
