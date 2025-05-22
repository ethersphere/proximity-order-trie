# Proximity Order Trie (POT)

A Go implementation of a Proximity Order Trie (POT) data structure, which is a specialized trie that organizes nodes based on their proximity order (bit-level similarity) to key values.

## Overview

A Proximity Order Trie is an efficient data structure for storing and retrieving data where keys can be compared by their proximity or similarity. Unlike traditional tries that branch on every character, POTs branch based on the proximity order of keys - the position of the first bit that differs between two keys.

This implementation supports:
- In-memory and persistent storage
- Flexible entry management
- Proximity-based node traversal and retrieval
- Configurable modes for different behaviors

## Concepts

### Key Components

- **Node**: Interface for trie nodes with methods for insertion, traversal, and manipulation
- **CNode**: A "cursored node" that captures a node viewed at a specific proximity order
- **Entry**: Interface for values stored in the trie, requiring methods for key access and serialization
- **Mode**: Interface that dictates the behavior of the trie (depth, insertion policy, persistence strategy)
- **Pottery**: A set of related faceted object relational mappings
- **Index**: A mutable pot with additional methods for updating and iterating

### Proximity Order

The proximity order (PO) between two byte sequences is determined by the position of the first bit that differs between them. The higher the PO, the more similar the keys are. This allows for efficient lookup and proximity-based retrieval.

## Usage

### Basic Usage

```go
// Create a basic in-memory POT with a maximum key length of 256 bits
mode := pot.NewSingleOrder(256)
root := mode.New()

// Create and insert entries
entry := YourEntryImplementation{...}
key := entry.Key()

// Update or insert an entry
updatedRoot, err := mode.Update(root, key, func(existing pot.Entry) pot.Entry {
    if existing == nil {
        return entry
    }
    // Merge or update logic
    return updatedEntry
})

// Find an entry
result, err := pot.Find(root, key, mode)
```

### Persistent Storage

```go
// Create a persistence-capable POT
persister := yourPersisterImplementation // that satisfies persister.LoadSaver
potMode := pot.NewPersistedPot(
    pot.NewSingleOrder(256),
    persister,
    func() pot.Entry { return &YourEntryType{} },
)

// Load existing POT
rootNode, loaded, err := potMode.Load(context.Background(), referenceBytes)

// Update and save
updatedRoot, err := potMode.Update(rootNode, key, updateFunction)
reference, err := potMode.Save(context.Background())
```

### Iteration

```go
// Iterate through entries in order of proximity to a key
err := pot.Iterate(startNode, prefix, targetKey, mode, func(entry pot.Entry) (bool, error) {
    // Process entry
    // Return true to stop iteration, false to continue
    return false, nil
})
```

## Customization

You can implement your own Entry types to store custom data, or extend the existing components with additional functionality.
