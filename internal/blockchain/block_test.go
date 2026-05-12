package blockchain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockHeader_CalculateHash(t *testing.T) {
	header := &BlockHeader{
		Height:       1,
		PrevHash:     Hash{1, 2, 3},
		Timestamp:    time.Now().Unix(),
		MerkleRoot:   Hash{4, 5, 6},
		TxCount:      2,
		Validator:    Address{1, 2, 3},
	}

	hash := header.CalculateHash()
	assert.NotEqual(t, Hash{}, hash, "Hash should not be empty")
	
	// Same header should produce same hash
	hash2 := header.CalculateHash()
	assert.Equal(t, hash, hash2, "Same header should have same hash")
	
	// Different header should produce different hash
	header.Height = 2
	hash3 := header.CalculateHash()
	assert.NotEqual(t, hash, hash3, "Different header should have different hash")
}

func TestBlock_HeaderCalculateHash(t *testing.T) {
	tx1 := &Transaction{
		From:  Address{1},
		To:    Address{2},
		Value: 100,
		Nonce: 1,
	}
	tx1.Hash = tx1.CalculateHash()

	block := &Block{
		Header: &BlockHeader{
			Height:    1,
			PrevHash:  Hash{1, 2, 3},
			Timestamp: time.Now().Unix(),
		},
		Transactions: []*Transaction{tx1},
	}

	hash := block.Header.CalculateHash()
	assert.NotEqual(t, Hash{}, hash, "Block hash should not be empty")
	
	// Verify hash is deterministic
	hash2 := block.Header.CalculateHash()
	assert.Equal(t, hash, hash2, "Block hash should be deterministic")
}

func TestBlock_VerifySignature(t *testing.T) {
	tx1 := &Transaction{
		From:  Address{1},
		To:    Address{2},
		Value: 100,
		Nonce: 1,
	}
	tx1.Hash = tx1.CalculateHash()

	block := &Block{
		Header: &BlockHeader{
			Height:    1,
			PrevHash:  Hash{1, 2, 3},
			Timestamp: time.Now().Unix(),
		},
		Transactions: []*Transaction{tx1},
	}

	// Without signature, verification should fail
	assert.False(t, block.VerifySignature(), "Block without signature should fail verification")
}

func TestTransaction_CalculateHash(t *testing.T) {
	tx := &Transaction{
		From:  Address{1},
		To:    Address{2},
		Value: 100,
		Nonce: 1,
		Data:  []byte("test"),
	}

	hash := tx.CalculateHash()
	assert.NotEqual(t, Hash{}, hash, "Transaction hash should not be empty")
	
	// Same transaction should produce same hash
	hash2 := tx.CalculateHash()
	assert.Equal(t, hash, hash2, "Same transaction should have same hash")
	
	// Different transaction should produce different hash
	tx.Value = 200
	hash3 := tx.CalculateHash()
	assert.NotEqual(t, hash, hash3, "Different transaction should have different hash")
}

func TestTransaction_VerifySignature(t *testing.T) {
	// Test with empty signature
	tx := &Transaction{
		From:  Address{1},
		To:    Address{2},
		Value: 100,
		Nonce: 1,
	}
	
	// Without signature, verification should fail
	assert.False(t, tx.VerifySignature(), "Unsigned transaction should fail verification")
}

func TestMerkleTree_CalculateRoot(t *testing.T) {
	// Test with even number of transactions
	tx1 := &Transaction{From: Address{1}, To: Address{2}, Value: 100, Nonce: 1}
	tx2 := &Transaction{From: Address{3}, To: Address{4}, Value: 200, Nonce: 2}
	tx1.Hash = tx1.CalculateHash()
	tx2.Hash = tx2.CalculateHash()

	txs := []*Transaction{tx1, tx2}
	root := CalculateMerkleRoot(txs)
	
	assert.NotEqual(t, Hash{}, root, "Merkle root should not be empty")
	
	// Test with odd number of transactions (should duplicate last)
	tx3 := &Transaction{From: Address{5}, To: Address{6}, Value: 300, Nonce: 3}
	tx3.Hash = tx3.CalculateHash()
	
	txs2 := []*Transaction{tx1, tx2, tx3}
	root2 := CalculateMerkleRoot(txs2)
	
	assert.NotEqual(t, Hash{}, root2, "Merkle root should not be empty")
	assert.NotEqual(t, root, root2, "Different transactions should have different root")
}

func TestMerkleTree_Empty(t *testing.T) {
	root := CalculateMerkleRoot([]*Transaction{})
	assert.Equal(t, Hash{}, root, "Empty tree should have zero hash")
}

func TestHash_String(t *testing.T) {
	h := Hash{0x01, 0x02, 0x03, 0x04}
	str := h.String()
	assert.Equal(t, "0102030400000000000000000000000000000000000000000000000000000000", str)
}

func TestAddress_String(t *testing.T) {
	a := Address{0x01, 0x02, 0x03, 0x04}
	str := a.String()
	assert.Equal(t, "0102030400000000000000000000000000000000", str)
}

func TestHexToHash(t *testing.T) {
	// Valid hex
	h, err := HexToHash("0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20")
	require.NoError(t, err)
	assert.Equal(t, byte(0x01), h[0])
	assert.Equal(t, byte(0x20), h[31])
	
	// Invalid hex
	_, err = HexToHash("invalid")
	assert.Error(t, err)
	
	// Wrong length
	_, err = HexToHash("0102")
	assert.Error(t, err)
}

func TestBlock_BlockSize(t *testing.T) {
	block := &Block{
		Header: &BlockHeader{
			Height:    1,
			Timestamp: time.Now().Unix(),
		},
		Transactions: []*Transaction{
			{From: Address{1}, To: Address{2}, Value: 100},
		},
	}
	
	size := block.BlockSize()
	assert.Greater(t, size, 0, "Block size should be greater than 0")
}
