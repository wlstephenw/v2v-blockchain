package consensus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
)

func TestPBFTMessage_SignAndVerify(t *testing.T) {
	// This is a simplified test - in real scenario would use actual keys
	msg := &PBFTMessage{
		Type:      MsgPrePrepare,
		View:      1,
		SeqNumber: 10,
		Sender:    blockchain.Address{1, 2, 3},
		Payload:   []byte("test payload"),
		Timestamp: time.Now().Unix(),
	}
	msg.Digest = msg.CalculateDigest()
	
	// Without signature, verification should fail
	assert.Error(t, msg.Verify(nil), "Message without signature should fail verification")
}

func TestPBFTMessage_CalculateDigest(t *testing.T) {
	msg := &PBFTMessage{
		Type:      MsgPrePrepare,
		View:      1,
		SeqNumber: 10,
		Sender:    blockchain.Address{1, 2, 3},
		Payload:   []byte("test payload"),
		Timestamp: time.Now().Unix(),
	}
	
	digest1 := msg.CalculateDigest()
	digest2 := msg.CalculateDigest()
	
	assert.Equal(t, digest1, digest2, "Digest should be deterministic")
	assert.NotEqual(t, blockchain.Hash{}, digest1, "Digest should not be empty")
	
	// Change message should change digest
	msg.SeqNumber = 11
	digest3 := msg.CalculateDigest()
	assert.NotEqual(t, digest1, digest3, "Different message should have different digest")
}

func TestViewNumber_NextPrimary(t *testing.T) {
	validators := []blockchain.Address{
		{1}, {2}, {3}, {4},
	}
	
	vs := &ValidatorSet{
		Validators: validators,
		ViewNumber: 0,
	}
	
	// View 0, primary should be first validator
	primary := vs.GetPrimary()
	assert.Equal(t, validators[0], primary, "View 0 primary should be first validator")
	
	// View 1, primary should be second validator
	vs.ViewNumber = 1
	primary = vs.GetPrimary()
	assert.Equal(t, validators[1], primary, "View 1 primary should be second validator")
	
	// View 4, should wrap around to first validator
	vs.ViewNumber = 4
	primary = vs.GetPrimary()
	assert.Equal(t, validators[0], primary, "View 4 primary should wrap to first validator")
}

func TestValidatorSet_IsValidator(t *testing.T) {
	validators := []blockchain.Address{
		{1}, {2}, {3}, {4},
	}
	
	vs := &ValidatorSet{
		Validators: validators,
	}
	
	assert.True(t, vs.IsValidator(validators[0]), "Should be validator")
	assert.False(t, vs.IsValidator(blockchain.Address{99}), "Should not be validator")
}

func TestRequestBatch_CalculateHash(t *testing.T) {
	tx1 := &blockchain.Transaction{From: blockchain.Address{1}, To: blockchain.Address{2}, Value: 100}
	tx2 := &blockchain.Transaction{From: blockchain.Address{3}, To: blockchain.Address{4}, Value: 200}
	
	batch := &RequestBatch{
		Transactions: []*blockchain.Transaction{tx1, tx2},
		Timestamp:    time.Now().Unix(),
	}
	
	hash1 := batch.CalculateHash()
	hash2 := batch.CalculateHash()
	
	assert.Equal(t, hash1, hash2, "Hash should be deterministic")
	assert.NotEqual(t, blockchain.Hash{}, hash1, "Hash should not be empty")
}

func TestPBFTConfig_Default(t *testing.T) {
	config := DefaultPBFTConfig()
	
	assert.Equal(t, 10*time.Second, config.ViewTimeout, "Default view timeout should be 10s")
	assert.Equal(t, 5*time.Second, config.RequestTimeout, "Default request timeout should be 5s")
	assert.Equal(t, uint64(100), config.CheckpointInterval, "Default checkpoint interval should be 100")
	assert.Equal(t, 2*time.Second, config.BlockInterval, "Default block interval should be 2s")
	assert.Equal(t, 100, config.MaxTxPerBlock, "Default max tx per block should be 100")
}

func TestCheckpoint(t *testing.T) {
	cp := &Checkpoint{
		SeqNumber: 100,
		BlockHash: blockchain.Hash{1, 2, 3},
		Timestamp: time.Now().Unix(),
	}
	
	assert.Equal(t, uint64(100), cp.SeqNumber, "Sequence number should match")
	assert.NotEqual(t, blockchain.Hash{}, cp.BlockHash, "Block hash should not be empty")
}
