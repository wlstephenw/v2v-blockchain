package txpool

import (
	"container/heap"
	"errors"
	"sync"
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

var (
	ErrTxAlreadyExists = errors.New("transaction already exists in pool")
	ErrTxPoolFull      = errors.New("transaction pool is full")
	ErrInvalidTx       = errors.New("invalid transaction")
)

// TxPoolConfig holds transaction pool configuration
type TxPoolConfig struct {
	MaxSize           int           // Maximum number of transactions
	MaxTxPerAccount   int           // Maximum transactions per account
	PriceBump         int           // Price bump percentage for replacement
	MaxTxLifetime     time.Duration // Maximum lifetime of a transaction
}

// DefaultTxPoolConfig returns default configuration
func DefaultTxPoolConfig() TxPoolConfig {
	return TxPoolConfig{
		MaxSize:         5000,
		MaxTxPerAccount: 100,
		PriceBump:       10,
		MaxTxLifetime:   30 * time.Minute,
	}
}

// TxPool manages pending transactions
type TxPool struct {
	config TxPoolConfig

	// All pending transactions
	pending map[blockchain.Hash]*TxItem
	all     map[blockchain.Address]map[uint64]*TxItem // by sender and nonce

	// Priority queues
	highPriority *PriorityQueue
	normalQueue  *PriorityQueue
	lowPriority  *PriorityQueue

	// Tracking
	mu sync.RWMutex

	// Lifecycle
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// TxItem wraps a transaction with metadata
type TxItem struct {
	Tx        *blockchain.Transaction
	AddedAt   time.Time
	Priority  int // Higher = more priority
}

// PriorityQueue implements heap.Interface for priority queue
type PriorityQueue []*TxItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// Higher priority first, then by timestamp
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	return pq[i].AddedAt.Before(pq[j].AddedAt)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*TxItem)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

// NewTxPool creates a new transaction pool
func NewTxPool(config TxPoolConfig) *TxPool {
	pool := &TxPool{
		config:       config,
		pending:      make(map[blockchain.Hash]*TxItem),
		all:          make(map[blockchain.Address]map[uint64]*TxItem),
		highPriority: &PriorityQueue{},
		normalQueue:  &PriorityQueue{},
		lowPriority:  &PriorityQueue{},
		stopCh:       make(chan struct{}),
	}

	heap.Init(pool.highPriority)
	heap.Init(pool.normalQueue)
	heap.Init(pool.lowPriority)

	// Start background tasks
	pool.wg.Add(1)
	go pool.cleanupLoop()

	logger.Info("Transaction pool created", logger.Int("max_size", config.MaxSize))
	return pool
}

// Stop stops the transaction pool
func (p *TxPool) Stop() {
	close(p.stopCh)
	p.wg.Wait()
	logger.Info("Transaction pool stopped")
}

// AddTx adds a transaction to the pool (Task 9.1, 9.2, 9.3)
func (p *TxPool) AddTx(tx *blockchain.Transaction, priority int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already exists (Task 9.3)
	if _, exists := p.pending[tx.Hash]; exists {
		return ErrTxAlreadyExists
	}

	// Check pool capacity
	if len(p.pending) >= p.config.MaxSize {
		return ErrTxPoolFull
	}

	// Validate transaction (Task 9.2)
	if err := p.validateTx(tx); err != nil {
		return err
	}

	// Create item
	item := &TxItem{
		Tx:        tx,
		AddedAt:   time.Now(),
		Priority:  priority,
	}

	// Add to pending
	p.pending[tx.Hash] = item

	// Add to sender's nonce map
	senderTxs, exists := p.all[tx.From]
	if !exists {
		senderTxs = make(map[uint64]*TxItem)
		p.all[tx.From] = senderTxs
	}
	senderTxs[tx.Nonce] = item

	// Add to priority queue
	if priority >= 10 {
		heap.Push(p.highPriority, item)
	} else if priority >= 5 {
		heap.Push(p.normalQueue, item)
	} else {
		heap.Push(p.lowPriority, item)
	}

	logger.Debug("Transaction added to pool",
		logger.String("hash", tx.Hash.String()),
		logger.String("from", tx.From.String()),
		logger.Int("priority", priority),
	)

	return nil
}

// validateTx validates a transaction
func (p *TxPool) validateTx(tx *blockchain.Transaction) error {
	// Check transaction hash
	calculatedHash := tx.CalculateHash()
	if !calculatedHash.Equals(tx.Hash) {
		return ErrInvalidTx
	}

	// Check signature
	if len(tx.Signature) > 0 {
		if !tx.VerifySignature() {
			return ErrInvalidTx
		}
	}

	// Check sender's transaction count
	senderTxs := p.all[tx.From]
	if len(senderTxs) >= p.config.MaxTxPerAccount {
		return ErrTxPoolFull
	}

	return nil
}

// GetPending returns pending transactions (Task 9.1)
func (p *TxPool) GetPending(max int) []*blockchain.Transaction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	txs := make([]*blockchain.Transaction, 0, max)

	// Get from high priority first
	for p.highPriority.Len() > 0 && len(txs) < max {
		item := heap.Pop(p.highPriority).(*TxItem)
		if _, exists := p.pending[item.Tx.Hash]; exists {
			txs = append(txs, item.Tx)
		}
	}

	// Then normal priority
	for p.normalQueue.Len() > 0 && len(txs) < max {
		item := heap.Pop(p.normalQueue).(*TxItem)
		if _, exists := p.pending[item.Tx.Hash]; exists {
			txs = append(txs, item.Tx)
		}
	}

	// Finally low priority
	for p.lowPriority.Len() > 0 && len(txs) < max {
		item := heap.Pop(p.lowPriority).(*TxItem)
		if _, exists := p.pending[item.Tx.Hash]; exists {
			txs = append(txs, item.Tx)
		}
	}

	return txs
}

// RemoveTx removes a transaction from the pool
func (p *TxPool) RemoveTx(hash blockchain.Hash) {
	p.mu.Lock()
	defer p.mu.Unlock()

	item, exists := p.pending[hash]
	if !exists {
		return
	}

	delete(p.pending, hash)

	// Remove from sender's map
	if senderTxs, exists := p.all[item.Tx.From]; exists {
		delete(senderTxs, item.Tx.Nonce)
		if len(senderTxs) == 0 {
			delete(p.all, item.Tx.From)
		}
	}
}

// HasTx checks if a transaction exists in the pool (Task 9.3)
func (p *TxPool) HasTx(hash blockchain.Hash) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	_, exists := p.pending[hash]
	return exists
}

// Size returns the number of pending transactions
func (p *TxPool) Size() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return len(p.pending)
}

// GetTx returns a transaction by hash
func (p *TxPool) GetTx(hash blockchain.Hash) (*blockchain.Transaction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	item, exists := p.pending[hash]
	if !exists {
		return nil, false
	}
	return item.Tx, true
}

// GetStats returns pool statistics
func (p *TxPool) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return map[string]interface{}{
		"pending_count":    len(p.pending),
		"high_priority":    p.highPriority.Len(),
		"normal_priority":  p.normalQueue.Len(),
		"low_priority":     p.lowPriority.Len(),
		"unique_senders":   len(p.all),
		"max_size":         p.config.MaxSize,
	}
}

// cleanupLoop periodically removes expired transactions
func (p *TxPool) cleanupLoop() {
	defer p.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			p.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired transactions
func (p *TxPool) cleanupExpired() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for hash, item := range p.pending {
		if now.Sub(item.AddedAt) > p.config.MaxTxLifetime {
			delete(p.pending, hash)

			// Remove from sender's map
			if senderTxs, exists := p.all[item.Tx.From]; exists {
				delete(senderTxs, item.Tx.Nonce)
				if len(senderTxs) == 0 {
					delete(p.all, item.Tx.From)
				}
			}

			expiredCount++
		}
	}

	if expiredCount > 0 {
		logger.Debug("Cleaned up expired transactions", logger.Int("count", expiredCount))
	}
}
