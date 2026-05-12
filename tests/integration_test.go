package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/platoon"
)

// TestBlockchainBasic tests basic blockchain operations (Task 10.10)
func TestBlockchainBasic(t *testing.T) {
	t.Run("BlockHash", func(t *testing.T) {
		header := &blockchain.BlockHeader{
			Height:    1,
			PrevHash:  blockchain.Hash{1, 2, 3},
			Timestamp: 1234567890,
		}
		hash := header.CalculateHash()
		assert.NotEqual(t, blockchain.Hash{}, hash)
	})

	t.Run("TransactionHash", func(t *testing.T) {
		tx := &blockchain.Transaction{
			From:  blockchain.Address{1},
			To:    blockchain.Address{2},
			Value: 100,
			Nonce: 1,
		}
		hash := tx.CalculateHash()
		assert.NotEqual(t, blockchain.Hash{}, hash)
	})

	t.Run("MerkleRoot", func(t *testing.T) {
		tx1 := &blockchain.Transaction{From: blockchain.Address{1}, Nonce: 1}
		tx2 := &blockchain.Transaction{From: blockchain.Address{2}, Nonce: 2}
		tx1.Hash = tx1.CalculateHash()
		tx2.Hash = tx2.CalculateHash()

		root := blockchain.CalculateMerkleRoot([]*blockchain.Transaction{tx1, tx2})
		assert.NotEqual(t, blockchain.Hash{}, root)
	})
}

// TestPlatoonBasic tests basic platoon operations
func TestPlatoonBasic(t *testing.T) {
	t.Run("PlatoonStatus", func(t *testing.T) {
		p := &platoon.Platoon{
			ID:     "test",
			Status: platoon.PlatoonStatusActive,
		}
		assert.True(t, p.IsActive())
		assert.Equal(t, "active", p.Status.String())
	})

	t.Run("PlatoonMembers", func(t *testing.T) {
		vehicle := blockchain.Address{1, 2, 3}
		p := &platoon.Platoon{
			ID: "test",
			Members: []*platoon.PlatoonMember{
				{VehicleID: vehicle},
			},
		}
		assert.True(t, p.HasMember(vehicle))
		assert.Equal(t, 1, p.GetMemberCount())
	})

	t.Run("PlatoonParams", func(t *testing.T) {
		params := platoon.PlatoonParams{
			MaxVehicles:  8,
			TargetSpeed:  30.0,
			SafeDistance: 20.0,
		}
		err := params.Validate()
		assert.NoError(t, err)

		// Invalid params
		params.MaxVehicles = 2
		err = params.Validate()
		assert.Error(t, err)
	})
}

// TestEndToEndWorkflow tests a complete workflow scenario
func TestEndToEndWorkflow(t *testing.T) {
	// Simulate: Create transaction -> Create block -> Verify

	// 1. Create transaction
	tx := &blockchain.Transaction{
		From:  blockchain.Address{1},
		To:    blockchain.Address{2},
		Value: 100,
		Nonce: 1,
		Type:  blockchain.TxTypeTransfer,
	}
	tx.Hash = tx.CalculateHash()
	assert.NotEqual(t, blockchain.Hash{}, tx.Hash)

	// 2. Create block with transaction
	block := &blockchain.Block{
		Header: &blockchain.BlockHeader{
			Height:    1,
			PrevHash:  blockchain.Hash{},
			Timestamp: 1234567890,
		},
		Transactions: []*blockchain.Transaction{tx},
	}
	block.Header.MerkleRoot = blockchain.CalculateMerkleRoot(block.Transactions)
	block.Header.Hash = block.Header.CalculateHash()

	// 3. Verify block structure
	assert.NotEqual(t, blockchain.Hash{}, block.Header.Hash)
	assert.NotEqual(t, blockchain.Hash{}, block.Header.MerkleRoot)
	assert.Equal(t, uint64(1), block.Header.Height)
	assert.Equal(t, 1, len(block.Transactions))

	// 4. Verify transaction in block
	assert.Equal(t, tx.Hash, block.Transactions[0].Hash)
	assert.Equal(t, uint64(100), block.Transactions[0].Value)
}
