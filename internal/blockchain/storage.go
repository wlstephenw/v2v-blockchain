package blockchain

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/v2v-blockchain/v2v-blockchain/internal/storage"
)

// BlockData represents the data stored for a block
type BlockData struct {
	HeaderHash   Hash              `json:"header_hash"`
	Transactions []*Transaction    `json:"transactions"`
}

// BlockStorage provides block storage operations
type BlockStorage struct {
	storage storage.Storage
}

// NewBlockStorage creates a new block storage
func NewBlockStorage(store storage.Storage) *BlockStorage {
	return &BlockStorage{storage: store}
}

// PutBlock stores a block
func (bs *BlockStorage) PutBlock(block *Block) error {
	batch := bs.storage.NewBatch()

	// Store block header
	headerKey := append(storage.PrefixBlockHeader, block.Header.Hash[:]...)
	headerData, err := json.Marshal(block.Header)
	if err != nil {
		return fmt.Errorf("failed to marshal block header: %w", err)
	}
	batch.Put(headerKey, headerData)

	// Store block body (transactions)
	blockKey := append(storage.PrefixBlock, block.Header.Hash[:]...)
	blockData := &BlockData{
		HeaderHash:   block.Header.Hash,
		Transactions: block.Transactions,
	}
	blockBytes, err := json.Marshal(blockData)
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}
	batch.Put(blockKey, blockBytes)

	// Store height -> hash mapping
	heightKey := make([]byte, 9)
	copy(heightKey, storage.PrefixBlockHeight)
	binary.BigEndian.PutUint64(heightKey[1:], block.Header.Height)
	batch.Put(heightKey, block.Header.Hash[:])

	// Store transactions
	for _, tx := range block.Transactions {
		txKey := append(storage.PrefixTransaction, tx.Hash[:]...)
		txData, err := json.Marshal(tx)
		if err != nil {
			return fmt.Errorf("failed to marshal transaction: %w", err)
		}
		batch.Put(txKey, txData)
	}

	return bs.storage.WriteBatch(batch)
}

// GetBlock retrieves a block by hash
func (bs *BlockStorage) GetBlock(hash Hash) (*Block, error) {
	// Get block data
	blockKey := append(storage.PrefixBlock, hash[:]...)
	blockData, err := bs.storage.Get(blockKey)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, fmt.Errorf("block not found: %s", hash.String())
		}
		return nil, err
	}

	var data BlockData
	if err := json.Unmarshal(blockData, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block: %w", err)
	}

	// Get block header
	headerKey := append(storage.PrefixBlockHeader, hash[:]...)
	headerData, err := bs.storage.Get(headerKey)
	if err != nil {
		return nil, fmt.Errorf("block header not found: %w", err)
	}

	var header BlockHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		return nil, fmt.Errorf("failed to unmarshal block header: %w", err)
	}

	return &Block{
		Header:       &header,
		Transactions: data.Transactions,
	}, nil
}

// GetBlockByHeight retrieves a block by height
func (bs *BlockStorage) GetBlockByHeight(height uint64) (*Block, error) {
	// Get hash from height mapping
	heightKey := make([]byte, 9)
	copy(heightKey, storage.PrefixBlockHeight)
	binary.BigEndian.PutUint64(heightKey[1:], height)

	hashBytes, err := bs.storage.Get(heightKey)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, fmt.Errorf("block not found at height: %d", height)
		}
		return nil, err
	}

	var hash Hash
	copy(hash[:], hashBytes)

	return bs.GetBlock(hash)
}

// GetLatestBlock retrieves the latest block
func (bs *BlockStorage) GetLatestBlock() (*Block, error) {
	// Get latest height from metadata
	height, err := bs.GetLatestHeight()
	if err != nil {
		return nil, err
	}

	if height == 0 {
		return nil, errors.New("no blocks in chain")
	}

	return bs.GetBlockByHeight(height)
}

// GetLatestHeight retrieves the latest block height
func (bs *BlockStorage) GetLatestHeight() (uint64, error) {
	data, err := bs.storage.Get(append(storage.PrefixMetadata, []byte("latest_height")...))
	if err != nil {
		if err == storage.ErrNotFound {
			return 0, nil // Genesis block doesn't exist yet
		}
		return 0, err
	}

	if len(data) != 8 {
		return 0, errors.New("invalid height data")
	}

	return binary.BigEndian.Uint64(data), nil
}

// SetLatestHeight sets the latest block height
func (bs *BlockStorage) SetLatestHeight(height uint64) error {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, height)
	return bs.storage.Put(append(storage.PrefixMetadata, []byte("latest_height")...), data)
}

// GetTransaction retrieves a transaction by hash
func (bs *BlockStorage) GetTransaction(hash Hash) (*Transaction, error) {
	txKey := append(storage.PrefixTransaction, hash[:]...)
	txData, err := bs.storage.Get(txKey)
	if err != nil {
		if err == storage.ErrNotFound {
			return nil, fmt.Errorf("transaction not found: %s", hash.String())
		}
		return nil, err
	}

	var tx Transaction
	if err := json.Unmarshal(txData, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}

	return &tx, nil
}

// HasBlock checks if a block exists
func (bs *BlockStorage) HasBlock(hash Hash) bool {
	blockKey := append(storage.PrefixBlock, hash[:]...)
	exists, _ := bs.storage.Has(blockKey)
	return exists
}

// HasTransaction checks if a transaction exists
func (bs *BlockStorage) HasTransaction(hash Hash) bool {
	txKey := append(storage.PrefixTransaction, hash[:]...)
	exists, _ := bs.storage.Has(txKey)
	return exists
}
