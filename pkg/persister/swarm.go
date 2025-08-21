package persister

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type SwarmLoadSaver struct {
	beeAPIURL string
	postageID []byte
	client    *http.Client
}

func NewSwarmLoadSaver(beeAPIURL string, postageID []byte) *SwarmLoadSaver {
	return &SwarmLoadSaver{
		beeAPIURL: beeAPIURL,
		postageID: postageID,
		client:    &http.Client{},
	}
}

// get beeapirul with error handling
func (sls *SwarmLoadSaver) getBeeAPIURL() (*url.URL, error) {
	u, err := url.Parse(sls.beeAPIURL)
	if err != nil {
		return nil, fmt.Errorf("invalid bee API URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("invalid bee API URL: scheme must be http or https")
	}
	if u.Host == "" {
		return nil, fmt.Errorf("invalid bee API URL: host is empty")
	}
	return u, nil
}

func (sls *SwarmLoadSaver) Load(ctx context.Context, reference []byte) ([]byte, error) {
	if len(reference) != 32 {
		return nil, fmt.Errorf("reference must be 32 bytes, got %d", len(reference))
	}

	refHex := fmt.Sprintf("%x", reference)
	u, err := sls.getBeeAPIURL()
	if err != nil {
		return nil, fmt.Errorf("invalid bee API URL: %w", err)
	}
	u.Path = fmt.Sprintf("/bytes/%s", refHex)
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := sls.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve data from swarm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("swarm returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return data, nil
}

func (sls *SwarmLoadSaver) Save(ctx context.Context, data []byte) ([]byte, error) {
	if len(sls.postageID) != 32 {
		return nil, fmt.Errorf("postage ID is not correct. Its length is %d", len(sls.postageID))
	}

	u, err := sls.getBeeAPIURL()
	if err != nil {
		return nil, fmt.Errorf("invalid bee API URL: %w", err)
	}
	u.Path = "/bytes"

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Swarm-Postage-Batch-Id", fmt.Sprintf("%x", sls.postageID))

	resp, err := sls.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to store data to swarm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("swarm returned status %d", resp.StatusCode)
	}

	// Read response to get the reference
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	var response struct {
		Reference string `json:"reference"`
	}
	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}
	refHex := response.Reference
	if len(refHex) != 64 { // 32 bytes * 2 hex chars per byte
		return nil, fmt.Errorf("invalid reference length: expected 64 hex chars, got %d", len(refHex))
	}
	reference, err := hex.DecodeString(refHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode reference hex: %w", err)
	}

	return reference, nil
}
