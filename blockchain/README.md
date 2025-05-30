# POT Blockchain
# Proximity Order Trie (POT) Proof Verification

This project contains the Solidity implementation of proof verification for the Proximity Order Trie (POT) data structure.

## Overview

The POT is a specialized trie structure optimized for proximity searches in a key space. This implementation provides smart contracts to verify the authenticity of data stored in a POT without requiring the entire trie to be stored on-chain.

## Proof System

The proof system enables verification of entry existence within the POT without storing the entire structure on-chain. It uses Binary Merkle Tree (BMT) proofs on Swarm Chunks (4KB data where the BMT root hash is hashed together with the chunk span).

### Proof Components

1. **ForkPathProof**: Contains all necessary proof segments to verify a path from the root to an entry
   - `rootReference`: The hash of the root node
   - `targetKey`: The key being verified
   - `forkRefProofs`: Array of fork reference proofs leading to the entry
   - `entryProof`: Proof for the actual entry

2. **ForkRefProof**: Verifies a fork reference
   - `bitVectorProof`: Proof for the bit vector (determines which children exist)
   - `forkReferenceProof`: Proof for the actual fork reference

3. **EntryProof**: Verifies an entry's existence
   - `bitVectorProof`: Proof for the node's bit vector
   - `entryProof`: Proof for the actual entry

## Development

Try running some of the following tasks:

```shell
npx hardhat help
npx hardhat test
REPORT_GAS=true npx hardhat test
npx hardhat node
#npx hardhat ignition deploy ./ignition/modules/POTProofVerifier.ts
```
