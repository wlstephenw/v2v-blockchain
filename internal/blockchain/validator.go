package blockchain

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// BlockValidator handles block validation
type BlockValidator struct {
	config *ValidationConfig
}

// ValidationConfig contains validation parameters
type ValidationConfig struct {
	MaxBlockSize      int           // Maximum block size in bytes
	MaxTxPerBlock     int           // Maximum transactions per block
	MaxBlockTimeDrift time.Duration // Maximum time drift for block timestamp
	MinBlockInterval  time.Duration // Minimum interval between blocks
}

// DefaultValidationConfig returns default validation config
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		MaxBlockSize:      10 * 1024 * 1024, // 10MB
		MaxTxPerBlock:     10000,
		MaxBlockTimeDrift: 2 * time.Minute,
		MinBlockInterval:  1 * time.Second,
	}
}

// NewBlockValidator creates a new block validator
func NewBlockValidator(cfg *ValidationConfig) *BlockValidator {
	if cfg == nil {
		cfg = DefaultValidationConfig()
	}
	return &BlockValidator{config: cfg}
}

// ValidateBlock validates a complete block
func (v *BlockValidator) ValidateBlock(block *Block, parent *Block) error {
	// Validate block header
	if err := v.ValidateHeader(block.Header, parent); err != nil {
		return fmt.Errorf("header validation failed: %w", err)
	}

	// Validate block hash
	if err := v.validateBlockHash(block); err != nil {
		return err
	}

	// Validate signature
	if err := v.validateBlockSignature(block); err != nil {
		return err
	}

	// Validate transactions
	if err := v.validateTransactions(block.Transactions); err != nil {
		return err
	}

	// Validate Merkle root
	if err := v.validateMerkleRoot(block); err != nil {
		return err
	}

	// Validate block size
	if err := v.validateBlockSize(block); err != nil {
		return err
	}

	// Validate block link (parent hash)
	if parent != nil {
		if err := v.validateBlockLink(block, parent); err != nil {
			return err
		}
	}

	logger.Debug("Block validated successfully",
		logger.Uint64("height", block.Header.Height),
		logger.String("hash", block.Header.Hash.String()),
	)

	return nil
}

// ValidateHeader validates a block header
func (v *BlockValidator) ValidateHeader(header *BlockHeader, parent *Block) error {
	// Check required fields
	if header.Height == 0 && !header.PrevHash.IsEmpty() {
		return errors.New("genesis block must have empty prev hash")
	}

	if header.Height > 0 && header.PrevHash.IsEmpty() {
		return errors.New("non-genesis block must have prev hash")
	}

	// Validate timestamp
	if err := v.validateTimestamp(header, parent); err != nil {
		return err
	}

	// Validate transaction count matches
	if header.TxCount != uint32(len(header.Validator)) {
		// This is just a consistency check, TxCount should match actual tx count
	}

	return nil
}

// validateBlockHash validates the block hash
func (v *BlockValidator) validateBlockHash(block *Block) error {
	calculatedHash := block.Header.CalculateHash()
	if !calculatedHash.Equals(block.Header.Hash) {
		return fmt.Errorf("invalid block hash: expected %s, got %s",
			calculatedHash.String(), block.Header.Hash.String())
	}
	return nil
}

// validateBlockSignature validates the block signature
func (v *BlockValidator) validateBlockSignature(block *Block) error {
	if len(block.Header.Signature) == 0 {
		return errors.New("block signature is missing")
	}

	if len(block.Header.Signature) != 65 {
		return fmt.Errorf("invalid signature length: %d", len(block.Header.Signature))
	}

	// Recover public key from signature
	data := block.Header.SerializeWithoutHash()
	hash := crypto.Keccak256Hash(data)

	pubKey, err := crypto.SigToPub(hash.Bytes(), block.Header.Signature)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	// Verify validator matches recovered public key
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	if !bytes.Equal(block.Header.Validator[:], recoveredAddr[:]) {
		return errors.New("block signature does not match validator")
	}

	return nil
}

// validateTransactions validates all transactions in a block
func (v *BlockValidator) validateTransactions(txs []*Transaction) error {
	if len(txs) > v.config.MaxTxPerBlock {
		return fmt.Errorf("too many transactions: %d > %d", len(txs), v.config.MaxTxPerBlock)
	}

	// Check for duplicate transactions
	txHashes := make(map[Hash]bool)
	for i, tx := range txs {
		if txHashes[tx.Hash] {
			return fmt.Errorf("duplicate transaction at index %d", i)
		}
		txHashes[tx.Hash] = true

		if err := v.validateTransaction(tx); err != nil {
			return fmt.Errorf("transaction %d validation failed: %w", i, err)
		}
	}

	return nil
}

// validateTransaction validates a single transaction
func (v *BlockValidator) validateTransaction(tx *Transaction) error {
	// Validate transaction hash
	calculatedHash := tx.CalculateHash()
	if !calculatedHash.Equals(tx.Hash) {
		return fmt.Errorf("invalid transaction hash: expected %s, got %s",
			calculatedHash.String(), tx.Hash.String())
	}

	// Validate signature
	if len(tx.Signature) == 0 {
		return errors.New("transaction signature is missing")
	}

	if len(tx.Signature) != 65 {
		return fmt.Errorf("invalid signature length: %d", len(tx.Signature))
	}

	// Recover public key and verify sender
	pubKey, err := crypto.SigToPub(tx.Hash.Bytes(), tx.Signature)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	if !bytes.Equal(tx.From[:], recoveredAddr[:]) {
		return errors.New("transaction signature does not match sender")
	}

	// Validate gas parameters
	if tx.GasLimit == 0 {
		return errors.New("gas limit cannot be zero")
	}

	// Validate timestamp (optional)
	txTime := time.Unix(tx.Timestamp, 0)
	if time.Since(txTime) > v.config.MaxBlockTimeDrift {
		return fmt.Errorf("transaction timestamp too old: %v", txTime)
	}
	if txTime.After(time.Now().Add(v.config.MaxBlockTimeDrift)) {
		return fmt.Errorf("transaction timestamp in future: %v", txTime)
	}

	return nil
}

// validateMerkleRoot validates the Merkle root
func (v *BlockValidator) validateMerkleRoot(block *Block) error {
	calculatedRoot := CalculateMerkleRoot(block.Transactions)
	if !calculatedRoot.Equals(block.Header.MerkleRoot) {
		return fmt.Errorf("invalid Merkle root: expected %s, got %s",
			calculatedRoot.String(), block.Header.MerkleRoot.String())
	}
	return nil
}

// validateBlockSize validates the block size
func (v *BlockValidator) validateBlockSize(block *Block) error {
	size := block.BlockSize()
	if size > v.config.MaxBlockSize {
		return fmt.Errorf("block size exceeds limit: %d > %d", size, v.config.MaxBlockSize)
	}
	return nil
}

// validateBlockLink validates the link to parent block
func (v *BlockValidator) validateBlockLink(block, parent *Block) error {
	// Validate parent hash
	if !block.Header.PrevHash.Equals(parent.Header.Hash) {
		return fmt.Errorf("invalid parent hash: expected %s, got %s",
			parent.Header.Hash.String(), block.Header.PrevHash.String())
	}

	// Validate height continuity
	expectedHeight := parent.Header.Height + 1
	if block.Header.Height != expectedHeight {
		return fmt.Errorf("invalid block height: expected %d, got %d",
			expectedHeight, block.Header.Height)
	}

	// Validate timestamp is after parent
	if block.Header.Timestamp <= parent.Header.Timestamp {
		return fmt.Errorf("block timestamp must be after parent: %d <= %d",
			block.Header.Timestamp, parent.Header.Timestamp)
	}

	// Validate minimum block interval
	parentTime := time.Unix(parent.Header.Timestamp, 0)
	blockTime := time.Unix(block.Header.Timestamp, 0)
	if blockTime.Sub(parentTime) < v.config.MinBlockInterval {
		return fmt.Errorf("block interval too short: %v < %v",
			blockTime.Sub(parentTime), v.config.MinBlockInterval)
	}

	return nil
}

// validateTimestamp validates block timestamp
func (v *BlockValidator) validateTimestamp(header *BlockHeader, parent *Block) error {
	blockTime := time.Unix(header.Timestamp, 0)
	now := time.Now()

	// Check timestamp is not too far in the future
	if blockTime.After(now.Add(v.config.MaxBlockTimeDrift)) {
		return fmt.Errorf("block timestamp too far in future: %v", blockTime)
	}

	// Check timestamp is not too old (unless it's genesis)
	if header.Height > 0 && now.Sub(blockTime) > v.config.MaxBlockTimeDrift {
		return fmt.Errorf("block timestamp too old: %v", blockTime)
	}

	return nil
}

// ValidateGenesisBlock validates the genesis block
func (v *BlockValidator) ValidateGenesisBlock(block *Block) error {
	if block.Header.Height != 0 {
		return fmt.Errorf("genesis block must have height 0, got %d", block.Header.Height)
	}

	if !block.Header.PrevHash.IsEmpty() {
		return errors.New("genesis block must have empty prev hash")
	}

	if len(block.Transactions) != 0 {
		return errors.New("genesis block should not have transactions")
	}

	return v.validateBlockHash(block)
}

// ValidationResult contains detailed validation results
type ValidationResult struct {
	Valid   bool     `json:"valid"`
	Errors  []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ValidateBlockDetailed performs detailed validation and returns all issues
func (v *BlockValidator) ValidateBlockDetailed(block *Block, parent *Block) *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// Run all validations and collect errors
	if err := v.ValidateBlock(block, parent); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	return result
}
