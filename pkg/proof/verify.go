package proof

import (
	"hash"

	"github.com/ethersphere/bee/v2/pkg/bmt"
	"golang.org/x/crypto/sha3"
)

// Verify returns the bmt hash obtained from the proof which can then be checked against
// the BMT hash of the chunk
func Verify(proof bmt.Proof) (root []byte, err error) {
	hasher := sha3.NewLegacyKeccak256()
	root = proof.ProveSegment
	i := proof.Index

	for _, sister := range proof.ProofSegments[:] {
		if i%2 == 0 {
			root, err = doHash(hasher, root, sister)
		} else {
			root, err = doHash(hasher, sister, root)
		}
		if err != nil {
			return nil, err
		}
		i >>= 1
	}
	return doHash(hasher, proof.Span, root)
}

// calculates Hash of the data
func doHash(h hash.Hash, data ...[]byte) ([]byte, error) {
	h.Reset()
	for _, v := range data {
		if _, err := h.Write(v); err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}
