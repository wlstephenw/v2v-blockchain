package blockchain

import (
	"errors"
	"fmt"
	"sync"

	"github.com/v2v-blockchain/v2v-blockchain/internal/config"
	"github.com/v2v-blockchain/v2v-blockchain/internal/storage"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

var timeNow = func() int64 { return 0 } // Placeholder for time.Now().Unix()

// Blockchain represents the blockchain
type Blockchain struct {
	config     *config.BlockchainConfig
	storage    storage.Storage
	blockStore *BlockStorage
	latestHash Hash
	latestHeight uint64
	mu         sync.RWMutex
}

// NewBlockchain creates a new blockchain instance
func NewBlockchain(cfg *config.BlockchainConfig, store storage.Storage) (*Blockchain, error) {
	bc := &Blockchain{
		config:     cfg,
		storage:    store,
		blockStore: NewBlockStorage(store),
	}

	// Try to get latest block info
	height, err := bc.blockStore.GetLatestHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest height: %w", err)
	}
	bc.latestHeight = height

	if height > 0 {
		latestBlock, err := bc.blockStore.GetLatestBlock()
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		bc.latestHash = latestBlock.Header.Hash
	}

	// Create genesis block if needed
	if bc.latestHeight == 0 && cfg.GenesisBlock {
		if err := bc.createGenesisBlock(); err != nil {
			return nil, fmt.Errorf("failed to create genesis block: %w", err)
		}
	}

	logger.Info("Blockchain initialized",
		logger.Uint64("latest_height", bc.latestHeight),
		logger.String("latest_hash", bc.latestHash.String()),
	)

	return bc, nil
}

// createGenesisBlock creates the genesis block
func (bc *Blockchain) createGenesisBlock() error {
	// Use a default validator address for genesis
	var genesisValidator Address

	genesisBlock := NewGenesisBlock(genesisValidator)

	if err := bc.AddBlock(genesisBlock); err != nil {
		return err
	}

	logger.Info("Genesis block created",
		logger.String("hash", genesisBlock.Header.Hash.String()),
	)

	return nil
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Validate block
	if err := bc.validateBlock(block); err != nil {
		return fmt.Errorf("block validation failed: %w", err)
	}

	// Store block
	if err := bc.storeBlock(block); err != nil {
		return fmt.Errorf("failed to store block: %w", err)
	}

	// Update latest info
	bc.latestHash = block.Header.Hash
	bc.latestHeight = block.Header.Height

	logger.Info("Block added to chain",
		logger.Uint64("height", block.Header.Height),
		logger.String("hash", block.Header.Hash.String()),
		logger.Int("tx_count", len(block.Transactions)),
	)

	return nil
}

// validateBlock validates a block before adding
func (bc *Blockchain) validateBlock(block *Block) error {
	// Check if block already exists
	if _, err := bc.GetBlockByHash(block.Header.Hash); err == nil {
		return errors.New("block already exists")
	}

	// Validate height
	expectedHeight := bc.latestHeight + 1
	if block.Header.Height != expectedHeight {
		return fmt.Errorf("invalid block height: expected %d, got %d", expectedHeight, block.Header.Height)
	}

	// Validate previous hash
	if block.Header.Height > 1 {
		if !block.Header.PrevHash.Equals(bc.latestHash) {
			return errors.New("invalid previous hash")
		}
	}

	// Validate block hash
	calculatedHash := block.Header.CalculateHash()
	if !calculatedHash.Equals(block.Header.Hash) {
		return errors.New("invalid block hash")
	}

	// Validate signature
	if len(block.Header.Signature) > 0 {
		if !block.VerifySignature() {
			return errors.New("invalid block signature")
		}
	}

	// Validate transactions
	for i, tx := range block.Transactions {
		if err := bc.validateTransaction(tx); err != nil {
			return fmt.Errorf("transaction %d validation failed: %w", i, err)
		}
	}

	// Validate Merkle root
	calculatedMerkleRoot := CalculateMerkleRoot(block.Transactions)
	if !calculatedMerkleRoot.Equals(block.Header.MerkleRoot) {
		return errors.New("invalid Merkle root")
	}

	// Validate block size
	if bc.config.MaxBlockSize > 0 && block.BlockSize() > bc.config.MaxBlockSize {
		return fmt.Errorf("block size exceeds maximum: %d > %d", block.BlockSize(), bc.config.MaxBlockSize)
	}

	// Validate transaction count
	if bc.config.MaxTxPerBlock > 0 && len(block.Transactions) > bc.config.MaxTxPerBlock {
		return fmt.Errorf("transaction count exceeds maximum: %d > %d", len(block.Transactions), bc.config.MaxTxPerBlock)
	}

	return nil
}

// validateTransaction validates a transaction
func (bc *Blockchain) validateTransaction(tx *Transaction) error {
	// Check if transaction already exists
	if _, err := bc.GetTransaction(tx.Hash); err == nil {
		return errors.New("transaction already exists")
	}

	// Validate transaction hash
	calculatedHash := tx.CalculateHash()
	if !calculatedHash.Equals(tx.Hash) {
		return errors.New("invalid transaction hash")
	}

	// Validate signature
	if len(tx.Signature) > 0 {
		if !tx.VerifySignature() {
			return errors.New("invalid transaction signature")
		}
	}

	return nil
}

// storeBlock stores a block in storage
func (bc *Blockchain) storeBlock(block *Block) error {
	if err := bc.blockStore.PutBlock(block); err != nil {
		return err
	}
	return bc.blockStore.SetLatestHeight(block.Header.Height)
}

// GetBlockByHash retrieves a block by hash
func (bc *Blockchain) GetBlockByHash(hash Hash) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.blockStore.GetBlock(hash)
}

// GetBlockByHeight retrieves a block by height
func (bc *Blockchain) GetBlockByHeight(height uint64) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.blockStore.GetBlockByHeight(height)
}

// GetLatestBlock retrieves the latest block
func (bc *Blockchain) GetLatestBlock() (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.latestHeight == 0 {
		return nil, errors.New("no blocks in chain")
	}

	return bc.GetBlockByHeight(bc.latestHeight)
}

// GetLatestHeight returns the latest block height
func (bc *Blockchain) GetLatestHeight() uint64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.latestHeight
}

// GetLatestHash returns the latest block hash
func (bc *Blockchain) GetLatestHash() Hash {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.latestHash
}

// GetTransaction retrieves a transaction by hash
func (bc *Blockchain) GetTransaction(hash Hash) (*Transaction, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.blockStore.GetTransaction(hash)
}

// GetBlocksRange retrieves blocks in a height range [start, end]
func (bc *Blockchain) GetBlocksRange(start, end uint64) ([]*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if start > end {
		return nil, errors.New("invalid range: start > end")
	}

	if end > bc.latestHeight {
		end = bc.latestHeight
	}

	blocks := make([]*Block, 0, end-start+1)
	for height := start; height <= end; height++ {
		block, err := bc.GetBlockByHeight(height)
		if err != nil {
			return nil, fmt.Errorf("failed to get block at height %d: %w", height, err)
		}
		blocks = append(blocks, block)
	}

	return blocks, nil
}

// HasBlock checks if a block exists
func (bc *Blockchain) HasBlock(hash Hash) bool {
	_, err := bc.GetBlockByHash(hash)
	return err == nil
}

// GetBlockCount returns the total number of blocks
func (bc *Blockchain) GetBlockCount() uint64 {
	return bc.GetLatestHeight()
}

// IsEmpty checks if the blockchain is empty
func (bc *Blockchain) IsEmpty() bool {
	return bc.GetLatestHeight() == 0
}

// CreateBlock creates a new block with the given transactions
func (bc *Blockchain) CreateBlock(txs []*Transaction, validator Address) (*Block, error) {
	bc.mu.RLock()
	latestHash := bc.latestHash
	latestHeight := bc.latestHeight
	bc.mu.RUnlock()

	block := NewBlock(latestHeight+1, latestHash, validator, txs)
	return block, nil
}
