// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "./BMTChunk.sol";

/**
 * @title POTProofVerifier
 * @notice Library to verify proofs of entries in a Proximity-Order-Trie
 * @dev The main function for contract importing the library is `assertForkPathProof`
 * Its workflow is the following:
 * Verifies Target Key Consistency: It first checks if the `targetKey` in the provided `proof` matches the key associated with the initial segment of the `entryProof`'s `bitVectorProof`. This ensures the proof is indeed for the claimed target.
 * Traverses Fork References: It iterates through the `forkRefProofs` array. For each fork:
 *   - It calculates the Proximity Order (PO) between the current fork's node key and the overall `targetKey`.
 *   - It verifies that the bit corresponding to this PO is set in the parent node's bitvector, confirming the fork's existence.
 *   - It determines the `forkRefSegmentIndex` based on the number of preceding forks (ones) in the bitvector.
 *   - It calls `assertForkRefProof` to validate the BMT proofs for both the bitvector of the current fork node and the actual fork reference (which is the hash of the child node).
 *   - The `currentNodeHash` is updated to the hash of the child node (the `proveSegment` of the `forkReferenceProof`) for the next iteration or for the final entry proof verification.
 * Verifies Final Entry: After processing all intermediate forks, it calls `assertEntryProof` using the final `currentNodeHash`.
 *   - `assertEntryProof` validates the BMT proof for the bitvector of the node containing the entry and the BMT proof for the entry itself. The `entrySegmentIndex` is calculated based on the number of forks in this final node's bitvector.
 */
library POTProofVerifier {
    // Maximum depth of the POT trie (256 bits)
    uint16 constant MAX_DEPTH = 256;
    uint8 constant BMT_SEGMENT_SIZE = 32;

    // segmentIndex is always 1 at bitVectorProof
    // segmentIndex is known from the bitVector at entryProof
    // segmentIndex is known from the bitVector and NodeKey/TargetKey PO at forkReferenceProof
    struct Proof {
        bytes32[] proofSegments;
        bytes32 proveSegment; // value that needs to be proved
        uint64 chunkSpan;
    }

    struct EntryProof {
        Proof bitVectorProof;
        Proof entryProof; // TODO: make arbitrary length bytes
    }

    struct ForkRefProof {
        Proof bitVectorProof;
        Proof forkReferenceProof;
    }

    // Full fork path proof structure
    struct ForkPathProof {
        bytes32 rootReference;
        bytes32 targetKey;
        ForkRefProof[] forkRefProofs;
        EntryProof entryProof;
    }

    /**
     * @notice Asserts a proof for a specific entry in the trie
     * @param proof The fork path proof containing all necessary proof segments
     * @dev Reverts if the proof is invalid
     */
    function assertForkPathProof(ForkPathProof calldata proof) internal pure {
        if (proof.entryProof.bitVectorProof.proofSegments[0] != proof.targetKey) {
            revert("Entry key does not match target key");
        }

        bytes32 currentNodeHash = proof.rootReference;
        uint16 calculatedPO = 0;
        
        for (uint i = 0; i < proof.forkRefProofs.length; i++) {
            bytes32 nodeKey = proof.forkRefProofs[i].bitVectorProof.proofSegments[0];
            bytes32 bitVector = proof.forkRefProofs[i].bitVectorProof.proveSegment;
            calculatedPO = calculatePO(nodeKey, proof.targetKey, uint8(calculatedPO));
            if(!isBitSet(bitVector, calculatedPO)) {
                revert("Fork is not set in the parent's bitvector");
            }
            uint16 forkIndex = countOnesInBitVectorUntil(bitVector, calculatedPO); // forks before
            uint16 forkRefSegmentIndex = 2 + forkIndex;
            assertForkRefProof(currentNodeHash, proof.forkRefProofs[i], forkRefSegmentIndex);

            currentNodeHash = proof.forkRefProofs[i].forkReferenceProof.proveSegment;
        }

        uint16 forkCount = countOnesInBitVectorUntil(proof.entryProof.bitVectorProof.proveSegment, MAX_DEPTH);
        uint16 forkDescendantsByteLength = forkCount * 4;
        uint16 entrySegmentIndex = (64 + forkCount * BMT_SEGMENT_SIZE + forkDescendantsByteLength) / 32;
        // padding after fork descendants' counts
        if (forkDescendantsByteLength%BMT_SEGMENT_SIZE != 0) {
            entrySegmentIndex++;
        }
        assertEntryProof(currentNodeHash, entrySegmentIndex, proof.entryProof);
    }

    /**
     * @notice Asserts a fork reference proof
     * @param nodeHash The hash of the current node
     * @param proof The fork reference proof to verify
     * @param forkRefSegmentIndex The segment index of the fork reference
     * @dev Reverts if the proof is invalid
     */
    function assertForkRefProof(
        bytes32 nodeHash,
        ForkRefProof calldata proof,
        uint16 forkRefSegmentIndex
    ) internal pure {
        bytes32 bitVectorHash = BMTChunk.chunkAddressFromInclusionProof(
            proof.bitVectorProof.proofSegments, 
            proof.bitVectorProof.proveSegment, 
            1, 
            proof.bitVectorProof.chunkSpan
        );
        if (bitVectorHash != nodeHash) {
            revert("Invalid bit vector proof at assertForkRefProof");
        }

        bytes32 forkRefHash = BMTChunk.chunkAddressFromInclusionProof(
            proof.forkReferenceProof.proofSegments,
            proof.forkReferenceProof.proveSegment,
            forkRefSegmentIndex,
            proof.forkReferenceProof.chunkSpan
        );
        if (forkRefHash != nodeHash) {
            revert("Invalid fork reference proof");
        }
    }

    /**
     * @notice Asserts an entry proof against a given node hash
     * @param nodeHash The hash of the node containing the entry
     * @param entrySegmentIndex The segment index of the entry
     * @param proof The entry proof to verify
     * @dev Reverts if the proof is invalid
     */
    function assertEntryProof(bytes32 nodeHash, uint16 entrySegmentIndex, EntryProof calldata proof) internal pure {
        bytes32 bitVectorHash = BMTChunk.chunkAddressFromInclusionProof(
            proof.bitVectorProof.proofSegments, 
            proof.bitVectorProof.proveSegment, 
            1, 
            proof.bitVectorProof.chunkSpan
        );
        if (bitVectorHash != nodeHash) {
            revert("Invalid bit vector proof at assertEntryProof");
        }
        
        bytes32 entryHash = BMTChunk.chunkAddressFromInclusionProof(
            proof.entryProof.proofSegments,
            proof.entryProof.proveSegment,
            entrySegmentIndex,
            proof.entryProof.chunkSpan
        );
        if (entryHash != nodeHash) {
            revert("Invalid entry proof");
        }
    }

    /**
     * @notice Calculates the proximity order (PO) between nodeKey and targetKey
     * @param nodeKey The key of the node
     * @param targetKey The target key being compared
     * @param pos The position to start comparing from
     * @return The proximity order (index of the first bit that differs)
     */
    function calculatePO(bytes32 nodeKey, bytes32 targetKey, uint8 pos) internal pure returns (uint16) {
        // Find the first bit that differs between nodeKey and targetKey
        uint8 bytePos = pos / 8;
        while (bytePos < MAX_DEPTH/8) {
            if (nodeKey[bytePos] != targetKey[bytePos]) {
                break;
            }
            bytePos++;
        }
        if (bytePos != MAX_DEPTH/8) {
            uint8 start = 0;
            if (bytePos == pos/8) {
                start = pos % 8;
            }
            uint8 oxo = uint8(nodeKey[bytePos]) ^ uint8(targetKey[bytePos]);
            for (uint8 j = start; j < 8; j++) {
                if ((oxo>>uint8(7-j))&0x01 != 0) {
                    return bytePos*8 + j;
                }
            }
        }

        return MAX_DEPTH;
    }
    
    /**
     * @notice Counts the number of set bits (1s) in a bitvector up to a specific index
     * @param bitVector The bitvector represented as bytes32
     * @param index The index up to which to count (not inclusive)
     * @return The count of set bits (1s) in the range [0, index)
     */
    function countOnesInBitVectorUntil(bytes32 bitVector, uint16 index) internal pure returns (uint16) {
        if (index == 0) {
            return 0;
        }
        
        uint16 count = 0;
        uint8 fullBytes = uint8(index / 8);
        // Process full bytes
        for (uint8 i = 0; i < fullBytes; i++) {
            uint8 b = uint8(bitVector[i]);
            // Count bits in this byte using Brian Kernighan's algorithm
            // This counts set bits in a byte more efficiently than checking each bit
            while (b > 0) {
                count++;
                b &= (b - 1); // Clear the least significant bit set
            }
        }
        
        // Process remaining bits in the partial byte if needed
        uint8 remainingBits = uint8(index % 8);
        if (remainingBits > 0) {
            uint8 b = uint8(bitVector[fullBytes]);
            // We only want to count bits up to the remainingBits position
            // Create a mask for the bits we care about (MSB first ordering)
            uint8 mask = uint8(0xFF) << (8 - remainingBits);
            b &= mask;
            
            while (b > 0) {
                count++;
                b &= (b - 1);
            }
        }
        
        return count;
    }
    
    /**
     * @notice Checks if a specific bit is set (1) in the bitvector
     * @param bitVector The bitvector represented as bytes32
     * @param index The specific bit index to check
     * @return True if the bit at the given index is set (1), false otherwise
     */
    function isBitSet(bytes32 bitVector, uint16 index) internal pure returns (bool) {
        if (index >= MAX_DEPTH) {
            return false;
        }
        
        uint8 bytePos = uint8(index / 8);
        uint8 bitPos = uint8(7 - (index % 8));
        uint8 b = uint8(bitVector[bytePos]);
        uint8 mask = uint8(1 << bitPos);
        
        return (b & mask) != 0;
    }
}
