package proof

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
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
func CreateForkPathProof(ctx context.Context, rootNode elements.Node, ls persister.LoadSaver, targetKey []byte) (*ForkPathProof, error) {
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

// return hexified JSON values used as smart contract validation parameter
func (f *ForkPathProof) JSON() string {
	proofsData := map[string]interface{}{
		"rootReference": "0x" + hex.EncodeToString(f.RootReference),
		"entryProof": map[string]interface{}{
			"entryProof": map[string]interface{}{
				"proveSegment": "0x" + hex.EncodeToString(f.EntryProof.EntryProof.ProveSegment),
				"proofSegments": func() []string {
					segments := make([]string, len(f.EntryProof.EntryProof.ProofSegments))
					for i, segment := range f.EntryProof.EntryProof.ProofSegments {
						segments[i] = "0x" + hex.EncodeToString(segment)
					}
					return segments
				}(),
				"chunkSpan": binary.LittleEndian.Uint64(f.EntryProof.EntryProof.Span),
			},
			"bitVectorProof": map[string]interface{}{
				"proveSegment": "0x" + hex.EncodeToString(f.EntryProof.BitVectorProof.ProveSegment),
				"proofSegments": func() []string {
					segments := make([]string, len(f.EntryProof.BitVectorProof.ProofSegments))
					for i, segment := range f.EntryProof.BitVectorProof.ProofSegments {
						segments[i] = "0x" + hex.EncodeToString(segment)
					}
					return segments
				}(),
				"chunkSpan": binary.LittleEndian.Uint64(f.EntryProof.BitVectorProof.Span),
			},
		},
		"forkRefProofs": func() []map[string]interface{} {
			forkRefProofs := make([]map[string]interface{}, len(f.ForkRefProofs))
			for i, forkRefProof := range f.ForkRefProofs {
				forkRefProofs[i] = map[string]interface{}{
					"forkReferenceProof": map[string]interface{}{
						"proveSegment": "0x" + hex.EncodeToString(forkRefProof.ForkReferenceProof.ProveSegment),
						"proofSegments": func() []string {
							segments := make([]string, len(forkRefProof.ForkReferenceProof.ProofSegments))
							for j, segment := range forkRefProof.ForkReferenceProof.ProofSegments {
								segments[j] = "0x" + hex.EncodeToString(segment)
							}
							return segments
						}(),
						"chunkSpan": binary.LittleEndian.Uint64(forkRefProof.ForkReferenceProof.Span),
					},
					"bitVectorProof": map[string]interface{}{
						"proveSegment": "0x" + hex.EncodeToString(forkRefProof.BitVectorProof.ProveSegment),
						"proofSegments": func() []string {
							segments := make([]string, len(forkRefProof.BitVectorProof.ProofSegments))
							for j, segment := range forkRefProof.BitVectorProof.ProofSegments {
								segments[j] = "0x" + hex.EncodeToString(segment)
							}
							return segments
						}(),
						"chunkSpan": binary.LittleEndian.Uint64(forkRefProof.BitVectorProof.Span),
					},
				}
			}
			return forkRefProofs
		}(),
		"targetKey": "0x" + hex.EncodeToString(f.TargetKey),
	}

	jsonProofsData, err := json.MarshalIndent(proofsData, "", "  ")
	if err != nil {
		return ""
	}
	return string(jsonProofsData)
}
