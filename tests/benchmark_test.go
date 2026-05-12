package tests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
)

// BenchmarkBlockHash benchmarks block header hash calculation (Task 13.9)
func BenchmarkBlockHash(b *testing.B) {
	header := &blockchain.BlockHeader{
		Height:    1000,
		PrevHash:  blockchain.Hash{1, 2, 3, 4, 5},
		Timestamp: time.Now().Unix(),
		Validator: blockchain.Address{1, 2, 3},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		header.CalculateHash()
	}
}

// BenchmarkTransactionHash benchmarks transaction hash calculation
func BenchmarkTransactionHash(b *testing.B) {
	tx := &blockchain.Transaction{
		From:  blockchain.Address{1, 2, 3},
		To:    blockchain.Address{4, 5, 6},
		Value: 100,
		Nonce: 1,
		Type:  blockchain.TxTypeTransfer,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx.CalculateHash()
	}
}

// BenchmarkMerkleRoot benchmarks Merkle tree root calculation
func BenchmarkMerkleRoot(b *testing.B) {
	// Create 100 transactions
	txs := make([]*blockchain.Transaction, 100)
	for i := 0; i < 100; i++ {
		txs[i] = &blockchain.Transaction{
			From:  blockchain.Address{byte(i)},
			To:    blockchain.Address{byte(i + 1)},
			Value: uint64(i * 10),
			Nonce: uint64(i),
		}
		txs[i].Hash = txs[i].CalculateHash()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blockchain.CalculateMerkleRoot(txs)
	}
}

// BenchmarkMerkleRootSmall benchmarks with 10 transactions
func BenchmarkMerkleRootSmall(b *testing.B) {
	txs := make([]*blockchain.Transaction, 10)
	for i := 0; i < 10; i++ {
		txs[i] = &blockchain.Transaction{
			From:  blockchain.Address{byte(i)},
			To:    blockchain.Address{byte(i + 1)},
			Value: uint64(i * 10),
			Nonce: uint64(i),
		}
		txs[i].Hash = txs[i].CalculateHash()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blockchain.CalculateMerkleRoot(txs)
	}
}

// BenchmarkMerkleRootLarge benchmarks with 1000 transactions
func BenchmarkMerkleRootLarge(b *testing.B) {
	txs := make([]*blockchain.Transaction, 1000)
	for i := 0; i < 1000; i++ {
		txs[i] = &blockchain.Transaction{
			From:  blockchain.Address{byte(i % 256)},
			To:    blockchain.Address{byte((i + 1) % 256)},
			Value: uint64(i * 10),
			Nonce: uint64(i),
		}
		txs[i].Hash = txs[i].CalculateHash()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blockchain.CalculateMerkleRoot(txs)
	}
}

// TestPerformanceRequirements verifies performance meets requirements
func TestPerformanceRequirements(t *testing.T) {
	t.Run("BlockHashLatency", func(t *testing.T) {
		header := &blockchain.BlockHeader{
			Height:    1000,
			PrevHash:  blockchain.Hash{1, 2, 3},
			Timestamp: time.Now().Unix(),
		}

		start := time.Now()
		iterations := 10000
		for i := 0; i < iterations; i++ {
			header.CalculateHash()
		}
		elapsed := time.Since(start)
		
		avgLatency := elapsed / time.Duration(iterations)
		t.Logf("Block hash average latency: %v", avgLatency)
		
		// Should be less than 1ms per hash
		assert.Less(t, avgLatency, time.Millisecond, "Block hash should be < 1ms")
	})

	t.Run("TransactionHashLatency", func(t *testing.T) {
		tx := &blockchain.Transaction{
			From:  blockchain.Address{1, 2, 3},
			To:    blockchain.Address{4, 5, 6},
			Value: 100,
			Nonce: 1,
		}

		start := time.Now()
		iterations := 10000
		for i := 0; i < iterations; i++ {
			tx.CalculateHash()
		}
		elapsed := time.Since(start)
		
		avgLatency := elapsed / time.Duration(iterations)
		t.Logf("Transaction hash average latency: %v", avgLatency)
		
		assert.Less(t, avgLatency, time.Millisecond, "Transaction hash should be < 1ms")
	})

	t.Run("MerkleRootLatency", func(t *testing.T) {
		// Test with typical block size (100 transactions)
		txs := make([]*blockchain.Transaction, 100)
		for i := 0; i < 100; i++ {
			txs[i] = &blockchain.Transaction{
				From:  blockchain.Address{byte(i)},
				To:    blockchain.Address{byte(i + 1)},
				Value: uint64(i * 10),
				Nonce: uint64(i),
			}
			txs[i].Hash = txs[i].CalculateHash()
		}

		start := time.Now()
		iterations := 1000
		for i := 0; i < iterations; i++ {
			blockchain.CalculateMerkleRoot(txs)
		}
		elapsed := time.Since(start)
		
		avgLatency := elapsed / time.Duration(iterations)
		t.Logf("Merkle root (100 txs) average latency: %v", avgLatency)
		
		// Should be less than 10ms for 100 transactions
		assert.Less(t, avgLatency, 10*time.Millisecond, "Merkle root should be < 10ms for 100 txs")
	})

	t.Run("ThroughputEstimate", func(t *testing.T) {
		// Estimate TPS based on transaction processing speed
		tx := &blockchain.Transaction{
			From:  blockchain.Address{1, 2, 3},
			To:    blockchain.Address{4, 5, 6},
			Value: 100,
			Nonce: 1,
		}

		start := time.Now()
		iterations := 100000
		for i := 0; i < iterations; i++ {
			tx.Nonce = uint64(i)
			tx.CalculateHash()
		}
		elapsed := time.Since(start)
		
		tps := float64(iterations) / elapsed.Seconds()
		t.Logf("Estimated TPS: %.0f", tps)
		
		// Should achieve at least 1000 TPS
		assert.Greater(t, tps, 1000.0, "Should achieve at least 1000 TPS")
	})
}
