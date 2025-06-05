[![CI](https://github.com/ethersphere/proximity-order-trie/actions/workflows/ci.yml/badge.svg)](https://github.com/ethersphere/proximity-order-trie/actions/workflows/ci.yml)

# Proximity Order Trie (POT)

_The go implementation is based on @zelig's [POT implementation](https://github.com/ethersphere/bee/tree/pottery)_

The Go implementation of the Proximity Order Trie (POT) data structure, which is a specialized trie that organizes nodes based on their proximity order (bit-level similarity) to key values.

## Overview

A Proximity Order Trie is an efficient data structure for storing and retrieving data where keys can be compared by their proximity or similarity. Unlike traditional tries that branch on every character, POTs branch based on the proximity order of keys - the position of the first bit that differs between two keys.

This implementation supports:
- In-memory and persistent Swarm storage
- Flexible entry management
- Proximity-based node traversal and retrieval
- Configurable modes for different behaviors

## Concepts

### Key Components

- **Node**: Interface for trie nodes with methods for insertion, traversal, and manipulation
- **CNode**: A "cursored node" that captures a node viewed at a specific proximity order
- **Entry**: Interface for values stored in the trie, requiring methods for key/value access and serialization
- **Mode**: Interface that dictates the behavior of the trie (depth, insertion policy, persistence strategy)
- **Index**: A mutable pot with additional methods for updating and iterating
- **KeyValueStore**: A key-value store interface for storing and retrieving data that supports Swarm storage

### Proximity Order

The proximity order (PO) between two byte sequences is determined by the position of the first bit that differs between them. The higher the PO, the more similar the keys are. This allows for efficient lookup and proximity-based retrieval.

## Usage

### Basic Usage

```go
// Create a new POT index with a standard mode
mode := elements.NewSingleOrder(256)
index, err := pot.New(mode)
if err != nil {
    panic(err)
}

// Create and insert an entry
ctx := context.Background()
entry := &pot.SwarmEntry{ // or your custom entry type
    key:   []byte("hello"),
    value: []byte("world"),
}
err = index.Add(ctx, entry)
if err != nil {
    panic(err)
}

// Update an existing entry
err = index.Update(ctx, []byte("hello"), func(e elements.Entry) elements.Entry {
    if e == nil {
        return nil // Entry not found
    }
    // Create a new entry with updated value
    return &pot.SwarmEntry{
        key:   e.Key(),
        value: []byte("updated world"),
    }
})

// Find an entry
found, err := index.Find(ctx, []byte("hello"))

```

### Iteration

```go
// Iterate through entries in order of proximity to a key
err := elements.Iterate(startNode, prefix, targetKey, mode, func(entry pot.Entry) (bool, error) {
    // Process entry
    // Return true to stop iteration, false to continue
    return false, nil
})
```

### KVS store

The Key-Value Store provides a simple interface for storing and retrieving data. It supports persistent storage through the Swarm storage backend and ensures thread-safe operations. The store leverages POT's proximity-based structure to enable efficient lookups and data retrieval.

```go
// Create a new KVS with a Swarm persister
ls := persister.NewInmemLoadSaver()
kvs, err := pot.NewSwarmKvs(ls)

// Store a value
ctx := context.Background()
err = kvs.Put(ctx, []byte("hello"), []byte("world"))

// Retrieve a value
value, err := kvs.Get(ctx, []byte("hello"))
if err != nil {
    if errors.Is(err, pot.ErrNotFound) {
        // Handle not found case
    }
    panic(err)
}
fmt.Println(string(value)) // "world"

// Persist the KVS to storage
reference, err := kvs.Save(ctx)

// Later, load the KVS from its reference
loadedKvs, err := pot.NewSwarmKvsReference(persister, reference)
```

### Index

Index provides a thread-safe, mutable POT interface with concurrent read access and exclusive write access:

```go
// Create a new mutable POT index
index, err := pot.New(mode)

// Add an entry to the index
err := index.Add(context.Background(), entry)

// Find an entry by key
result, err := index.Find(context.Background(), key)

// Delete an entry by key
err := index.Delete(context.Background(), key)

// Update an entry using a function
err := index.Update(context.Background(), key, func(existing pot.Entry) pot.Entry {
    // Your update logic here
    return updatedEntry
})

// Iterate through entries near a key
err := index.Iterate(prefix, targetKey, func(entry pot.Entry) (bool, error) {
    // Process entry
    // Return true to stop iteration, false to continue
    return false, nil
})

// Persist the index
ref, err := index.Save(context.Background())
```

## Proof System & Blockchain Integration

The POT implementation includes a proof generation and verification system that enables trustless verification of data inclusion without requiring the entire trie structure to be available. It uses Binary Merkle Tree (BMT) proofs on Swarm Chunks (4KB data where the BMT root hash is hashed together with the chunk span).

### Proof Components

- **ForkPathProof**: A complete proof for a path from the root to an entry, containing:
  - Root reference (hash of the root node)
  - Target key being proven
  - Array of fork reference proofs for each node in the path
  - Entry proof for the final data

- **Proof Verification**: The `blockchain/` directory contains Solidity smart contracts that can verify POT proofs on-chain, enabling blockchain applications to trustlessly verify data from a POT without storing the entire structure.

Example of generating and verifying a proof:

```go
import "github.com/ethersphere/proximity-order-trie/pkg/proof"

rootNode := index.GetRootNode()
ls := persister.NewInmemLoadSaver()
key := make([]byte, 32)
copy(key[24:], []byte{0xb0, 0xba, 0xf3, 0x77})

// Generate a proof for a key
proof, err := proof.CreateForkPathProof(rootNode, ls, key)

// On the blockchain side, the proof can be verified using the POTProofVerifier contract
// See blockchain/README.md for more details on the verification process
```

## Customization

You can implement your own Entry types to store custom data, or extend the existing components with additional functionality.
