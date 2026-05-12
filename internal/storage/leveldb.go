package storage

import (
	"errors"
	"fmt"
	"os"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/iterator"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
	"github.com/v2v-blockchain/v2v-blockchain/internal/config"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// LevelDBStorage implements blockchain storage using LevelDB
type LevelDBStorage struct {
	db     *leveldb.DB
	path   string
	config config.StorageConfig
}

// NewLevelDBStorage creates a new LevelDB storage instance
func NewLevelDBStorage(cfg config.StorageConfig) (*LevelDBStorage, error) {
	// Ensure directory exists
	if err := os.MkdirAll(cfg.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Configure LevelDB options
	options := &opt.Options{
		BlockCacheCapacity:     cfg.CacheSize * opt.MiB,
		OpenFilesCacheCapacity: 64,
		Filter:                 filter.NewBloomFilter(10),
		Compression:            opt.SnappyCompression,
	}

	if !cfg.Compression {
		options.Compression = opt.NoCompression
	}

	// Open database
	db, err := leveldb.OpenFile(cfg.Path, options)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	storage := &LevelDBStorage{
		db:     db,
		path:   cfg.Path,
		config: cfg,
	}

	logger.Info("LevelDB storage initialized",
		logger.String("path", cfg.Path),
		logger.Int("cache_size", cfg.CacheSize),
	)

	return storage, nil
}

// Close closes the database
func (s *LevelDBStorage) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// Put stores a key-value pair
func (s *LevelDBStorage) Put(key, value []byte) error {
	return s.db.Put(key, value, nil)
}

// Get retrieves a value by key
func (s *LevelDBStorage) Get(key []byte) ([]byte, error) {
	value, err := s.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, ErrNotFound
	}
	return value, err
}

// Delete removes a key-value pair
func (s *LevelDBStorage) Delete(key []byte) error {
	return s.db.Delete(key, nil)
}

// Has checks if a key exists
func (s *LevelDBStorage) Has(key []byte) (bool, error) {
	return s.db.Has(key, nil)
}

// NewIterator creates a new iterator
func (s *LevelDBStorage) NewIterator(prefix []byte) Iterator {
	rangePrefix := util.BytesPrefix(prefix)
	return &levelDBIterator{
		iter: s.db.NewIterator(rangePrefix, nil),
	}
}

// WriteBatch writes a batch of operations atomically
func (s *LevelDBStorage) WriteBatch(batch Batch) error {
	ldbBatch := &leveldb.Batch{}

	// Copy operations from our batch to LevelDB batch
	for _, op := range batch.(*levelDBBatch).ops {
		switch op.opType {
		case opPut:
			ldbBatch.Put(op.key, op.value)
		case opDelete:
			ldbBatch.Delete(op.key)
		}
	}

	return s.db.Write(ldbBatch, nil)
}

// NewBatch creates a new batch
func (s *LevelDBStorage) NewBatch() Batch {
	return &levelDBBatch{}
}

// Common errors
var ErrNotFound = errors.New("key not found")

// Prefixes for different data types
var (
	PrefixBlock        = []byte("b") // Block data
	PrefixBlockHeader  = []byte("h") // Block header
	PrefixBlockHeight  = []byte("n") // Block number -> hash mapping
	PrefixTransaction  = []byte("t") // Transaction data
	PrefixState        = []byte("s") // State data
	PrefixMetadata     = []byte("m") // Metadata
	PrefixIdentity     = []byte("i") // Identity data
	PrefixPlatoon      = []byte("p") // Platoon data
	PrefixAuditLog     = []byte("a") // Audit log
)

// DB returns the underlying LevelDB instance (for use by other packages)
func (s *LevelDBStorage) DB() *leveldb.DB {
	return s.db
}


// levelDBIterator wraps LevelDB iterator
type levelDBIterator struct {
	iter iterator.Iterator
}

func (i *levelDBIterator) Next() bool {
	return i.iter.Next()
}

func (i *levelDBIterator) Key() []byte {
	return i.iter.Key()
}

func (i *levelDBIterator) Value() []byte {
	return i.iter.Value()
}

func (i *levelDBIterator) Error() error {
	return i.iter.Error()
}

func (i *levelDBIterator) Release() {
	i.iter.Release()
}

// Operation types for batch
type opType byte

const (
	opPut opType = iota
	opDelete
)

type batchOp struct {
	opType opType
	key    []byte
	value  []byte
}

// levelDBBatch implements Batch interface
type levelDBBatch struct {
	ops []batchOp
}

func (b *levelDBBatch) Put(key, value []byte) {
	b.ops = append(b.ops, batchOp{
		opType: opPut,
		key:    append([]byte{}, key...),
		value:  append([]byte{}, value...),
	})
}

func (b *levelDBBatch) Delete(key []byte) {
	b.ops = append(b.ops, batchOp{
		opType: opDelete,
		key:    append([]byte{}, key...),
	})
}

func (b *levelDBBatch) Reset() {
	b.ops = b.ops[:0]
}

func (b *levelDBBatch) Len() int {
	return len(b.ops)
}

// Storage interface
type Storage interface {
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Delete(key []byte) error
	Has(key []byte) (bool, error)
	NewIterator(prefix []byte) Iterator
	WriteBatch(batch Batch) error
	NewBatch() Batch
	Close() error
}

// Iterator interface
type Iterator interface {
	Next() bool
	Key() []byte
	Value() []byte
	Error() error
	Release()
}

// Batch interface
type Batch interface {
	Put(key, value []byte)
	Delete(key []byte)
	Reset()
	Len() int
}

// Ensure LevelDBStorage implements Storage interface
var _ Storage = (*LevelDBStorage)(nil)
