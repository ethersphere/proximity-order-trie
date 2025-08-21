//go:build swarm_integration

package persister_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ethersphere/proximity-order-trie/pkg/persister"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultBeeAPIURL = "http://localhost:1633"
	testTimeout      = 30 * time.Second
)

func TestSwarmLoadSaver_Integration(t *testing.T) {
	beeAPIURL := os.Getenv("BEE_API_URL")
	if beeAPIURL == "" {
		beeAPIURL = defaultBeeAPIURL
	}

	postageID := os.Getenv("BEE_BATCH_ID")
	if postageID == "" {
		t.Fatalf("BEE_BATCH_ID environment variable not set, skipping swarm integration tests")
	}
	postageIDBytes, err := hex.DecodeString(postageID)
	if err != nil {
		t.Fatalf("failed to decode postage ID hex string: %v", postageID)
	}

	t.Run("Save and Load different data types", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sls := persister.NewSwarmLoadSaver(beeAPIURL, postageIDBytes)

		largeData := make([]byte, 8193)
		_, err := rand.Read(largeData)
		require.NoError(t, err, "Failed to generate random test data")

		for _, tc := range []struct {
			name string
			data []byte
		}{
			{"empty data", []byte{}},
			{"little data", []byte("This is a little test string with more content to test data handling in Swarm storage.")},
			{"large random data", largeData},
		} {
			t.Run(tc.name, func(t *testing.T) {
				fmt.Println("Saving", tc.name)
				reference, err := sls.Save(ctx, tc.data)
				require.NoError(t, err, "Failed to save %s to Swarm", tc.name)
				require.NotNil(t, reference, "Reference should not be nil")
				require.Len(t, reference, 32, "Reference should be 32 bytes")

				loadedData, err := sls.Load(ctx, reference)
				require.NoError(t, err, "Failed to load %s from Swarm", tc.name)
				require.NotNil(t, loadedData, "Loaded data should not be nil")

				assert.Equal(t, tc.data, loadedData, "Loaded %s should match original", tc.name)
			})
		}
	})

	t.Run("Concurrent Save and Load", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		sls := persister.NewSwarmLoadSaver(beeAPIURL, postageIDBytes)

		const numGoroutines = 5
		const numOperations = 3

		results := make(chan error, numGoroutines*numOperations*2) // *2 for save+load

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < numOperations; j++ {
					testData := []byte(fmt.Sprintf("Goroutine %d, Operation %d", goroutineID, j))

					reference, err := sls.Save(ctx, testData)
					if err != nil {
						results <- fmt.Errorf("goroutine %d save %d failed: %w", goroutineID, j, err)
						continue
					}
					results <- nil // Save success

					// Load
					loadedData, err := sls.Load(ctx, reference)
					if err != nil {
						results <- fmt.Errorf("goroutine %d load %d failed: %w", goroutineID, j, err)
						continue
					}

					if !assert.ObjectsAreEqual(testData, loadedData) {
						results <- fmt.Errorf("goroutine %d data mismatch %d", goroutineID, j)
						continue
					}
					results <- nil // Load success
				}
			}(i)
		}

		for i := 0; i < len(results); i++ {
			err := <-results
			if err != nil {
				t.Errorf("Concurrent operation failed: %v", err)
			}
		}
	})
}

func TestSwarmLoadSaver_ErrorCases(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	t.Run("Save without postage ID", func(t *testing.T) {
		sls := persister.NewSwarmLoadSaver(defaultBeeAPIURL, []byte{})

		testData := []byte("test data")
		_, err := sls.Save(ctx, testData)

		require.Error(t, err, "Save should fail without postage ID")
		assert.Contains(t, err.Error(), "postage ID is not correct")
	})

	t.Run("Invalid Bee API URL", func(t *testing.T) {
		sls := persister.NewSwarmLoadSaver("invalid-url", make([]byte, 32))

		testData := []byte("test data")
		_, err := sls.Save(ctx, testData)

		require.Error(t, err, "Save should fail with invalid URL")
		assert.Contains(t, err.Error(), "invalid bee API URL")
	})

	t.Run("Load with invalid reference length", func(t *testing.T) {
		sls := persister.NewSwarmLoadSaver(defaultBeeAPIURL, []byte{})

		// Test with wrong reference length
		invalidRef := []byte("short")
		_, err := sls.Load(ctx, invalidRef)

		require.Error(t, err, "Load should fail with invalid reference length")
		assert.Contains(t, err.Error(), "reference must be 32 bytes")
	})

	t.Run("Load with non-existent reference", func(t *testing.T) {
		sls := persister.NewSwarmLoadSaver(defaultBeeAPIURL, []byte{})

		// Create a fake 32-byte reference that doesn't exist
		fakeRef := make([]byte, 32)
		for i := range fakeRef {
			fakeRef[i] = 0xFF
		}

		_, err := sls.Load(ctx, fakeRef)

		require.Error(t, err, "swarm returned status 404")
	})
}
