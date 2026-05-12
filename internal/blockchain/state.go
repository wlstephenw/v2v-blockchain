package blockchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/v2v-blockchain/v2v-blockchain/internal/storage"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// ChainState manages the current state of the blockchain
type ChainState struct {
	mu           sync.RWMutex
	latestHeight uint64
	latestHash   Hash
	latestBlock  *Block
	totalTxCount uint64
	difficulty   uint64

	// Caches
	blockCache   map[Hash]*Block
	txCache      map[Hash]*Transaction
	maxCacheSize int

	// Storage reference
	storage storage.Storage
}

// NewChainState creates a new chain state manager
func NewChainState(store storage.Storage) (*ChainState, error) {
	cs := &ChainState{
		storage:      store,
		blockCache:   make(map[Hash]*Block),
		txCache:      make(map[Hash]*Transaction),
		maxCacheSize: 1000,
	}

	// Load state from storage
	if err := cs.loadState(); err != nil {
		logger.Warn("Failed to load chain state, starting fresh", logger.ErrField(err))
	}

	return cs, nil
}

// loadState loads the chain state from storage
func (cs *ChainState) loadState() error {
	blockStore := NewBlockStorage(cs.storage)

	height, err := blockStore.GetLatestHeight()
	if err != nil {
		return err
	}

	cs.latestHeight = height

	if height > 0 {
		latestBlock, err := blockStore.GetLatestBlock()
		if err != nil {
			return err
		}
		cs.latestHash = latestBlock.Header.Hash
		cs.latestBlock = latestBlock

		// Load total transaction count
		cs.totalTxCount = cs.calculateTotalTxCount()
	}

	return nil
}

// calculateTotalTxCount calculates the total number of transactions
func (cs *ChainState) calculateTotalTxCount() uint64 {
	// This is a simplified version - in production, you'd maintain a counter
	var count uint64
	blockStore := NewBlockStorage(cs.storage)

	for height := uint64(1); height <= cs.latestHeight; height++ {
		block, err := blockStore.GetBlockByHeight(height)
		if err != nil {
			continue
		}
		count += uint64(len(block.Transactions))
	}
	return count
}

// UpdateLatestBlock updates the latest block state
func (cs *ChainState) UpdateLatestBlock(block *Block) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.latestHeight = block.Header.Height
	cs.latestHash = block.Header.Hash
	cs.latestBlock = block
	cs.totalTxCount += uint64(len(block.Transactions))

	// Add to cache
	cs.addBlockToCache(block)

	// Cache transactions
	for _, tx := range block.Transactions {
		cs.addTxToCache(tx)
	}

	logger.Debug("Chain state updated",
		logger.Uint64("height", cs.latestHeight),
		logger.String("hash", cs.latestHash.String()),
	)
}

// GetLatestHeight returns the latest block height
func (cs *ChainState) GetLatestHeight() uint64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.latestHeight
}

// GetLatestHash returns the latest block hash
func (cs *ChainState) GetLatestHash() Hash {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.latestHash
}

// GetLatestBlock returns the latest block
func (cs *ChainState) GetLatestBlock() *Block {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.latestBlock
}

// GetTotalTxCount returns the total transaction count
func (cs *ChainState) GetTotalTxCount() uint64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.totalTxCount
}

// GetDifficulty returns the current difficulty
func (cs *ChainState) GetDifficulty() uint64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return cs.difficulty
}

// SetDifficulty sets the current difficulty
func (cs *ChainState) SetDifficulty(difficulty uint64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.difficulty = difficulty
}

// GetBlockFromCache retrieves a block from cache
func (cs *ChainState) GetBlockFromCache(hash Hash) (*Block, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	block, ok := cs.blockCache[hash]
	return block, ok
}

// GetTxFromCache retrieves a transaction from cache
func (cs *ChainState) GetTxFromCache(hash Hash) (*Transaction, bool) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	tx, ok := cs.txCache[hash]
	return tx, ok
}

// addBlockToCache adds a block to the cache
func (cs *ChainState) addBlockToCache(block *Block) {
	if len(cs.blockCache) >= cs.maxCacheSize {
		// Remove oldest entry (simple eviction)
		for k := range cs.blockCache {
			delete(cs.blockCache, k)
			break
		}
	}
	cs.blockCache[block.Header.Hash] = block
}

// addTxToCache adds a transaction to the cache
func (cs *ChainState) addTxToCache(tx *Transaction) {
	if len(cs.txCache) >= cs.maxCacheSize {
		// Remove oldest entry
		for k := range cs.txCache {
			delete(cs.txCache, k)
			break
		}
	}
	cs.txCache[tx.Hash] = tx
}

// ClearCache clears all caches
func (cs *ChainState) ClearCache() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.blockCache = make(map[Hash]*Block)
	cs.txCache = make(map[Hash]*Transaction)
}

// GetCacheStats returns cache statistics
func (cs *ChainState) GetCacheStats() (blockCount, txCount int) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return len(cs.blockCache), len(cs.txCache)
}

// StateSnapshot represents a snapshot of the chain state
type StateSnapshot struct {
	LatestHeight uint64 `json:"latest_height"`
	LatestHash   string `json:"latest_hash"`
	TotalTxCount uint64 `json:"total_tx_count"`
	Difficulty   uint64 `json:"difficulty"`
	Timestamp    int64  `json:"timestamp"`
}

// GetSnapshot returns a snapshot of the current state
func (cs *ChainState) GetSnapshot() *StateSnapshot {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return &StateSnapshot{
		LatestHeight: cs.latestHeight,
		LatestHash:   cs.latestHash.String(),
		TotalTxCount: cs.totalTxCount,
		Difficulty:   cs.difficulty,
		Timestamp:    0, // Placeholder
	}
}

// SaveSnapshot saves the current state snapshot to storage
func (cs *ChainState) SaveSnapshot() error {
	snapshot := cs.GetSnapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	key := append([]byte("state_"), []byte("snapshot")...)
	return cs.storage.Put(key, data)
}

// LoadSnapshot loads a state snapshot from storage
func (cs *ChainState) LoadSnapshot() (*StateSnapshot, error) {
	key := append([]byte("state_"), []byte("snapshot")...)
	data, err := cs.storage.Get(key)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, errors.New("no snapshot found")
		}
		return nil, err
	}

	var snapshot StateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot: %w", err)
	}

	return &snapshot, nil
}


// ChainStats represents blockchain statistics
type ChainStats struct {
	BlockCount      uint64  `json:"block_count"`
	TransactionCount uint64 `json:"transaction_count"`
	AvgBlockTime    float64 `json:"avg_block_time_ms"`
	AvgTxPerBlock   float64 `json:"avg_tx_per_block"`
}

// GetStats returns blockchain statistics
func (cs *ChainState) GetStats() *ChainStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	avgTxPerBlock := float64(0)
	if cs.latestHeight > 0 {
		avgTxPerBlock = float64(cs.totalTxCount) / float64(cs.latestHeight)
	}

	return &ChainStats{
		BlockCount:       cs.latestHeight,
		TransactionCount: cs.totalTxCount,
		AvgTxPerBlock:    avgTxPerBlock,
	}
}

// Reset resets the chain state (use with caution)
func (cs *ChainState) Reset() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.latestHeight = 0
	cs.latestHash = Hash{}
	cs.latestBlock = nil
	cs.totalTxCount = 0
	cs.difficulty = 0
	cs.blockCache = make(map[Hash]*Block)
	cs.txCache = make(map[Hash]*Transaction)

	logger.Warn("Chain state has been reset")
}
