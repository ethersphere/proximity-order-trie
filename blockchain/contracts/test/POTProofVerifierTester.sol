// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "../POTProofVerifier.sol";

/**
 * @title POTProofVerifierTester
 * @notice Contract to test POTProofVerifier library functions
 */
contract POTProofVerifierTester {
    /**
     * @notice Public wrapper for the library's assertForkPathProof function
     */
    function assertForkPathProof(POTProofVerifier.ForkPathProof calldata proof) public pure {
        POTProofVerifier.assertForkPathProof(proof);
    }

    /**
     * @notice Public wrapper for the library's calculatePO function
     */
    function calculatePOPublic(bytes32 nodeKey, bytes32 targetKey, uint8 pos) public pure returns (uint16) {
        return POTProofVerifier.calculatePO(nodeKey, targetKey, pos);
    }

    /**
     * @notice Public wrapper for the library's countOnesInBitVectorUntil function
     */
    function countOnesInBitVectorUntilPublic(bytes32 bitVector, uint16 index) public pure returns (uint16) {
        return POTProofVerifier.countOnesInBitVectorUntil(bitVector, index);
    }

    /**
     * @notice Public wrapper for the library's isBitSet function
     */
    function isBitSetPublic(bytes32 bitVector, uint16 index) public pure returns (bool) {
        return POTProofVerifier.isBitSet(bitVector, index);
    }

    /**
     * @notice Public wrapper for the library's assertForkRefProof function
     */
    function assertForkRefProofPublic(
        bytes32 nodeHash,
        POTProofVerifier.ForkRefProof calldata proof,
        uint16 forkRefSegmentIndex
    ) public pure {
        POTProofVerifier.assertForkRefProof(nodeHash, proof, forkRefSegmentIndex);
    }

    /**
     * @notice Public wrapper for the library's assertEntryProof function
     */
    function assertEntryProofPublic(
        bytes32 nodeHash,
        uint16 entrySegmentIndex,
        POTProofVerifier.EntryProof calldata proof
    ) public pure {
        POTProofVerifier.assertEntryProof(nodeHash, entrySegmentIndex, proof);
    }
}
