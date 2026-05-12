package blockchain

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// ForkDetector handles fork detection and resolution
type ForkDetector struct {
	mu          sync.RWMutex
	forks       map[uint64][]*Fork // height -> forks at that height
	mainChain   *Blockchain
	config      *ForkConfig
}

// Fork represents a detected fork
type Fork struct {
	Height      uint64     `json:"height"`
	Hash        Hash       `json:"hash"`
	ParentHash  Hash       `json:"parent_hash"`
	Timestamp   int64      `json:"timestamp"`
	Difficulty  uint64     `json:"difficulty"`
	IsMainChain bool       `json:"is_main_chain"`
	Children    []Hash     `json:"children,omitempty"`
}

// ForkConfig contains fork detection configuration
type ForkConfig struct {
	MaxForkDepth        int    // Maximum depth to track forks
	ConfirmationDepth   int    // Blocks needed to confirm main chain
	ReorgLimit          int    // Maximum reorganization allowed
}

// DefaultForkConfig returns default fork configuration
func DefaultForkConfig() *ForkConfig {
	return &ForkConfig{
		MaxForkDepth:      100,
		ConfirmationDepth: 6,
		ReorgLimit:        50,
	}
}

// NewForkDetector creates a new fork detector
func NewForkDetector(mainChain *Blockchain, cfg *ForkConfig) *ForkDetector {
	if cfg == nil {
		cfg = DefaultForkConfig()
	}

	return &ForkDetector{
		forks:     make(map[uint64][]*Fork),
		mainChain: mainChain,
		config:    cfg,
	}
}

// DetectFork checks if adding a block would create a fork
func (fd *ForkDetector) DetectFork(block *Block) (*Fork, error) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	// Check if this block's parent is on the main chain
	parentHeight := block.Header.Height - 1
	if parentHeight == 0 {
		return nil, nil // Genesis block can't create forks
	}

	// Get parent from main chain
	_, err := fd.mainChain.GetBlockByHash(block.Header.PrevHash)
	if err != nil {
		// Parent not on main chain - might be extending a fork
		logger.Debug("Parent not on main chain, possible fork extension",
			logger.String("parent_hash", block.Header.PrevHash.String()),
		)
	}
	_ = err // Suppress unused variable warning for now

	// Check if there's already a block at this height on main chain
	existingBlock, err := fd.mainChain.GetBlockByHeight(block.Header.Height)
	if err == nil && !existingBlock.Header.Hash.Equals(block.Header.Hash) {
		// This is a fork!
		fork := &Fork{
			Height:      block.Header.Height,
			Hash:        block.Header.Hash,
			ParentHash:  block.Header.PrevHash,
			Timestamp:   block.Header.Timestamp,
			Difficulty:  1, // Simplified
			IsMainChain: false,
		}

		fd.forks[block.Header.Height] = append(fd.forks[block.Header.Height], fork)

		logger.Info("Fork detected",
			logger.Uint64("height", block.Header.Height),
			logger.String("hash", block.Header.Hash.String()),
			logger.String("main_hash", existingBlock.Header.Hash.String()),
		)

		return fork, nil
	}

	return nil, nil
}

// ResolveFork attempts to resolve a fork by selecting the best chain
func (fd *ForkDetector) ResolveFork(height uint64) (*Fork, error) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	forks, ok := fd.forks[height]
	if !ok || len(forks) == 0 {
		return nil, errors.New("no forks at this height")
	}

	// Get main chain block at this height
	mainBlock, err := fd.mainChain.GetBlockByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to get main chain block: %w", err)
	}

	// Calculate total difficulty for each fork
	type forkScore struct {
		fork  *Fork
		score uint64
	}

	scores := []forkScore{
		{fork: &Fork{Height: height, Hash: mainBlock.Header.Hash}, score: fd.calculateChainScore(mainBlock.Header.Hash)},
	}

	for _, fork := range forks {
		score := fd.calculateChainScore(fork.Hash)
		scores = append(scores, forkScore{fork: fork, score: score})
	}

	// Sort by score (descending)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	bestFork := scores[0].fork

	// Check if we need to reorg
	if !bestFork.Hash.Equals(mainBlock.Header.Hash) {
		logger.Info("Fork resolution suggests reorganization",
			logger.Uint64("height", height),
			logger.String("new_best", bestFork.Hash.String()),
			logger.String("current_main", mainBlock.Header.Hash.String()),
		)
	}

	return bestFork, nil
}

// calculateChainScore calculates the score of a chain (simplified - uses height)
func (fd *ForkDetector) calculateChainScore(hash Hash) uint64 {
	// In a real implementation, this would calculate total difficulty
	// For now, we use a simplified approach
	return 1
}

// HandleReorganization performs chain reorganization
func (fd *ForkDetector) HandleReorganization(newHead Hash) error {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	// Find common ancestor
	commonAncestor, oldChain, newChain, err := fd.findCommonAncestor(newHead)
	if err != nil {
		return fmt.Errorf("failed to find common ancestor: %w", err)
	}

	// Check reorg depth
	if len(oldChain) > fd.config.ReorgLimit {
		return fmt.Errorf("reorganization too deep: %d > %d", len(oldChain), fd.config.ReorgLimit)
	}

	logger.Info("Chain reorganization",
		logger.String("common_ancestor", commonAncestor.String()),
		logger.Int("old_chain_length", len(oldChain)),
		logger.Int("new_chain_length", len(newChain)),
	)

	// Perform reorg (in a real implementation, this would update the main chain)
	// For now, we just log it

	return nil
}

// findCommonAncestor finds the common ancestor of two chains
func (fd *ForkDetector) findCommonAncestor(newHead Hash) (Hash, []Hash, []Hash, error) {
	// Build map of blocks in new chain
	newChain := []Hash{newHead}
	current := newHead

	for {
		block, err := fd.mainChain.GetBlockByHash(current)
		if err != nil {
			// Try to get from forks
			break
		}

		if block.Header.Height == 0 {
			break
		}

		current = block.Header.PrevHash
		newChain = append(newChain, current)
	}

	// Build map of blocks in old chain
	oldChain := []Hash{}
	latestHash := fd.mainChain.GetLatestHash()
	current = latestHash

	for {
		// Check if current is in new chain
		for _, h := range newChain {
			if h.Equals(current) {
				// Found common ancestor
				return current, oldChain, newChain, nil
			}
		}

		block, err := fd.mainChain.GetBlockByHash(current)
		if err != nil {
			break
		}

		if block.Header.Height == 0 {
			break
		}

		oldChain = append(oldChain, current)
		current = block.Header.PrevHash
	}

	return Hash{}, nil, nil, errors.New("no common ancestor found")
}

// GetForksAtHeight returns all forks at a given height
func (fd *ForkDetector) GetForksAtHeight(height uint64) []*Fork {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	forks, ok := fd.forks[height]
	if !ok {
		return nil
	}

	// Return copy
	result := make([]*Fork, len(forks))
	copy(result, forks)
	return result
}

// GetAllForks returns all detected forks
func (fd *ForkDetector) GetAllForks() []*Fork {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	var allForks []*Fork
	for _, forks := range fd.forks {
		allForks = append(allForks, forks...)
	}

	return allForks
}

// IsConfirmed checks if a block is confirmed (deep enough in the chain)
func (fd *ForkDetector) IsConfirmed(height uint64) bool {
	latestHeight := fd.mainChain.GetLatestHeight()
	return latestHeight-height >= uint64(fd.config.ConfirmationDepth)
}

// PruneOldForks removes forks that are too old
func (fd *ForkDetector) PruneOldForks() int {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	latestHeight := fd.mainChain.GetLatestHeight()
	pruned := 0

	for height := range fd.forks {
		if int(latestHeight-height) > fd.config.MaxForkDepth {
			delete(fd.forks, height)
			pruned++
		}
	}

	if pruned > 0 {
		logger.Debug("Pruned old forks", logger.Int("count", pruned))
	}

	return pruned
}

// GetForkStats returns fork statistics
type ForkStats struct {
	TotalForks      int            `json:"total_forks"`
	MaxForkHeight   uint64         `json:"max_fork_height"`
	ActiveForks     int            `json:"active_forks"`
	ResolvedForks   int            `json:"resolved_forks"`
	ForksByHeight   map[uint64]int `json:"forks_by_height"`
}

// GetStats returns fork statistics
func (fd *ForkDetector) GetStats() *ForkStats {
	fd.mu.RLock()
	defer fd.mu.RUnlock()

	stats := &ForkStats{
		TotalForks:    0,
		ForksByHeight: make(map[uint64]int),
	}

	for height, forks := range fd.forks {
		stats.TotalForks += len(forks)
		stats.ForksByHeight[height] = len(forks)

		if height > stats.MaxForkHeight {
			stats.MaxForkHeight = height
		}

		// Count active forks (not on main chain)
		for _, fork := range forks {
			if !fork.IsMainChain {
				stats.ActiveForks++
			}
		}
	}

	return stats
}

// AddFork manually adds a fork (for testing or recovery)
func (fd *ForkDetector) AddFork(fork *Fork) {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	fd.forks[fork.Height] = append(fd.forks[fork.Height], fork)

	logger.Debug("Fork manually added",
		logger.Uint64("height", fork.Height),
		logger.String("hash", fork.Hash.String()),
	)
}

// ClearForks clears all fork data (use with caution)
func (fd *ForkDetector) ClearForks() {
	fd.mu.Lock()
	defer fd.mu.Unlock()

	fd.forks = make(map[uint64][]*Fork)

	logger.Warn("All fork data cleared")
}
