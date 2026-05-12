package blockchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/v2v-blockchain/v2v-blockchain/internal/infra/storage"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// LightClient represents a light client that only stores block headers
type LightClient struct {
	mu          sync.RWMutex
	headers     map[uint64]*BlockHeader
	hashToHeight map[Hash]uint64
	latestHeight uint64
	latestHash   Hash
	storage      storage.Storage
	config       *LightClientConfig
}

// LightClientConfig contains light client configuration
type LightClientConfig struct {
	MaxHeadersInMemory int  // Maximum number of headers to keep in memory
	VerifySignatures   bool // Whether to verify block signatures
	VerifyTimestamps   bool // Whether to verify timestamps
}

// DefaultLightClientConfig returns default light client config
func DefaultLightClientConfig() *LightClientConfig {
	return &LightClientConfig{
		MaxHeadersInMemory: 10000,
		VerifySignatures:   true,
		VerifyTimestamps:   true,
	}
}

// NewLightClient creates a new light client
func NewLightClient(store storage.Storage, cfg *LightClientConfig) (*LightClient, error) {
	if cfg == nil {
		cfg = DefaultLightClientConfig()
	}

	lc := &LightClient{
		headers:      make(map[uint64]*BlockHeader),
		hashToHeight: make(map[Hash]uint64),
		storage:      store,
		config:       cfg,
	}

	// Load existing headers from storage
	if err := lc.loadHeaders(); err != nil {
		logger.Warn("Failed to load headers from storage", logger.ErrField(err))
	}

	logger.Info("Light client initialized",
		logger.Int("headers_in_memory", len(lc.headers)),
		logger.Uint64("latest_height", lc.latestHeight),
	)

	return lc, nil
}

// loadHeaders loads headers from storage
func (lc *LightClient) loadHeaders() error {
	// This is a simplified version - in production, you'd load from storage
	return nil
}

// AddHeader adds a new block header
func (lc *LightClient) AddHeader(header *BlockHeader) error {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	// Validate header
	if err := lc.validateHeader(header); err != nil {
		return fmt.Errorf("header validation failed: %w", err)
	}

	// Store header
	lc.headers[header.Height] = header
	lc.hashToHeight[header.Hash] = header.Height

	// Update latest
	if header.Height > lc.latestHeight {
		lc.latestHeight = header.Height
		lc.latestHash = header.Hash
	}

	// Evict old headers if needed
	lc.evictOldHeaders()

	// Persist to storage
	if err := lc.persistHeader(header); err != nil {
		logger.Warn("Failed to persist header", logger.ErrField(err))
	}

	logger.Debug("Header added to light client",
		logger.Uint64("height", header.Height),
		logger.String("hash", header.Hash.String()),
	)

	return nil
}

// validateHeader validates a block header for light client
func (lc *LightClient) validateHeader(header *BlockHeader) error {
	// Check if header already exists
	if existing, ok := lc.headers[header.Height]; ok {
		if existing.Hash.Equals(header.Hash) {
			return errors.New("header already exists")
		}
		// Different hash at same height - potential fork
		return fmt.Errorf("conflicting header at height %d", header.Height)
	}

	// Validate height continuity
	if header.Height > 0 {
		if _, ok := lc.headers[header.Height-1]; !ok {
			return fmt.Errorf("parent header not found for height %d", header.Height)
		}
	}

	// Validate previous hash link
	if header.Height > 0 {
		parent := lc.headers[header.Height-1]
		if !header.PrevHash.Equals(parent.Hash) {
			return errors.New("invalid previous hash link")
		}
	}

	// Verify block hash
	calculatedHash := header.CalculateHash()
	if !calculatedHash.Equals(header.Hash) {
		return errors.New("invalid block hash")
	}

	// Verify signature if enabled
	if lc.config.VerifySignatures && len(header.Signature) > 0 {
		if err := lc.verifyHeaderSignature(header); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
	}

	return nil
}

// verifyHeaderSignature verifies the block signature
func (lc *LightClient) verifyHeaderSignature(header *BlockHeader) error {
	// Create a temporary block for signature verification
	block := &Block{Header: header}
	if !block.VerifySignature() {
		return errors.New("invalid block signature")
	}
	return nil
}

// evictOldHeaders removes old headers when memory limit is reached
func (lc *LightClient) evictOldHeaders() {
	if len(lc.headers) <= lc.config.MaxHeadersInMemory {
		return
	}

	// Remove oldest headers (keep most recent)
	toRemove := len(lc.headers) - lc.config.MaxHeadersInMemory
	for height := uint64(1); toRemove > 0 && height < lc.latestHeight; height++ {
		if header, ok := lc.headers[height]; ok {
			delete(lc.headers, height)
			delete(lc.hashToHeight, header.Hash)
			toRemove--
		}
	}
}

// persistHeader saves header to storage
func (lc *LightClient) persistHeader(header *BlockHeader) error {
	data, err := json.Marshal(header)
	if err != nil {
		return err
	}

	key := append([]byte("light_header_"), []byte(fmt.Sprintf("%d", header.Height))...)
	return lc.storage.Put(key, data)
}

// GetHeader retrieves a header by height
func (lc *LightClient) GetHeader(height uint64) (*BlockHeader, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	header, ok := lc.headers[height]
	if !ok {
		return nil, fmt.Errorf("header not found at height %d", height)
	}

	return header, nil
}

// GetHeaderByHash retrieves a header by hash
func (lc *LightClient) GetHeaderByHash(hash Hash) (*BlockHeader, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	height, ok := lc.hashToHeight[hash]
	if !ok {
		return nil, fmt.Errorf("header not found for hash %s", hash.String())
	}

	return lc.headers[height], nil
}

// GetLatestHeader returns the latest header
func (lc *LightClient) GetLatestHeader() *BlockHeader {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return lc.headers[lc.latestHeight]
}

// GetLatestHeight returns the latest block height
func (lc *LightClient) GetLatestHeight() uint64 {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return lc.latestHeight
}

// GetLatestHash returns the latest block hash
func (lc *LightClient) GetLatestHash() Hash {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return lc.latestHash
}

// GetHeadersRange retrieves headers in a range [start, end]
func (lc *LightClient) GetHeadersRange(start, end uint64) ([]*BlockHeader, error) {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	if start > end {
		return nil, errors.New("invalid range: start > end")
	}

	if end > lc.latestHeight {
		end = lc.latestHeight
	}

	headers := make([]*BlockHeader, 0, end-start+1)
	for height := start; height <= end; height++ {
		if header, ok := lc.headers[height]; ok {
			headers = append(headers, header)
		}
	}

	return headers, nil
}

// HasHeader checks if a header exists
func (lc *LightClient) HasHeader(height uint64) bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	_, ok := lc.headers[height]
	return ok
}

// HasHeaderByHash checks if a header with given hash exists
func (lc *LightClient) HasHeaderByHash(hash Hash) bool {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	_, ok := lc.hashToHeight[hash]
	return ok
}

// GetHeaderCount returns the number of headers stored
func (lc *LightClient) GetHeaderCount() int {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return len(lc.headers)
}

// VerifyTransaction verifies a transaction is included in a block (simplified)
func (lc *LightClient) VerifyTransaction(txHash Hash, blockHeight uint64, merkleProof *MerkleProof) (bool, error) {
	header, err := lc.GetHeader(blockHeight)
	if err != nil {
		return false, err
	}

	if merkleProof == nil {
		return false, errors.New("merkle proof required for verification")
	}

	// Verify Merkle proof
	if !merkleProof.Verify(header.MerkleRoot) {
		return false, errors.New("invalid merkle proof")
	}

	return true, nil
}

// SyncStatus represents the sync status of the light client
type SyncStatus struct {
	LatestHeight    uint64 `json:"latest_height"`
	StoredHeaders   int    `json:"stored_headers"`
	IsSynced        bool   `json:"is_synced"`
	TargetHeight    uint64 `json:"target_height,omitempty"`
}

// GetSyncStatus returns the current sync status
func (lc *LightClient) GetSyncStatus() *SyncStatus {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	return &SyncStatus{
		LatestHeight:  lc.latestHeight,
		StoredHeaders: len(lc.headers),
		IsSynced:      true, // Light client is always "synced" to what it has
	}
}

// Reset clears all headers (use with caution)
func (lc *LightClient) Reset() {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	lc.headers = make(map[uint64]*BlockHeader)
	lc.hashToHeight = make(map[Hash]uint64)
	lc.latestHeight = 0
	lc.latestHash = Hash{}

	logger.Warn("Light client headers reset")
}

// LightClientStats contains statistics about the light client
type LightClientStats struct {
	TotalHeaders    int    `json:"total_headers"`
	LatestHeight    uint64 `json:"latest_height"`
	MemoryUsed      int    `json:"memory_used_bytes"`
	OldestHeight    uint64 `json:"oldest_height"`
}

// GetStats returns light client statistics
func (lc *LightClient) GetStats() *LightClientStats {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	stats := &LightClientStats{
		TotalHeaders: len(lc.headers),
		LatestHeight: lc.latestHeight,
	}

	// Find oldest height
	if len(lc.headers) > 0 {
		oldest := lc.latestHeight
		for height := range lc.headers {
			if height < oldest {
				oldest = height
			}
		}
		stats.OldestHeight = oldest
	}

	// Estimate memory usage (rough)
	stats.MemoryUsed = len(lc.headers) * 400 // ~400 bytes per header

	return stats
}
