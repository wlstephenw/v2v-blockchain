package message

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
)

func TestV2VMessage_Verify(t *testing.T) {
	msg := &V2VMessage{
		ID:        "msg-001",
		Type:      MsgTypeControl,
		Priority:  PriorityHigh,
		Sender:    blockchain.Address{1, 2, 3},
		Receiver:  blockchain.Address{4, 5, 6},
		SeqNum:    1,
		Timestamp: time.Now().Unix(),
		Payload:   []byte("test payload"),
		PlatoonID: "platoon-001",
	}
	
	// Without signature, verification should fail
	assert.Error(t, msg.Verify(), "Message without signature should fail verification")
}

func TestV2VMessage_IsExpired(t *testing.T) {
	// Message within time window
	msg := &V2VMessage{
		Timestamp: time.Now().Unix(),
	}
	assert.False(t, msg.IsExpired(), "Recent message should not be expired")
	
	// Message too old
	msg.Timestamp = time.Now().Add(-2 * time.Minute).Unix()
	assert.True(t, msg.IsExpired(), "Old message should be expired")
	
	// Message in future
	msg.Timestamp = time.Now().Add(2 * time.Minute).Unix()
	assert.True(t, msg.IsExpired(), "Future message should be expired")
}

func TestMessageType_String(t *testing.T) {
	assert.Equal(t, "control", MsgTypeControl.String())
	assert.Equal(t, "status", MsgTypeStatus.String())
	assert.Equal(t, "emergency", MsgTypeEmergency.String())
	assert.Equal(t, "unknown", MessageType(99).String())
}

func TestPriority(t *testing.T) {
	// Verify priority constants exist
	_ = PriorityLow
	_ = PriorityNormal
	_ = PriorityHigh
	_ = PriorityEmergency
}

func TestSequenceManager_Next(t *testing.T) {
	sm := NewSequenceManager()
	
	// First sequence number should be 1
	seq1 := sm.Next()
	assert.Equal(t, uint64(1), seq1, "First sequence should be 1")
	
	// Next should increment
	seq2 := sm.Next()
	assert.Equal(t, uint64(2), seq2, "Second sequence should be 2")
	
	// Should be monotonic
	seq3 := sm.Next()
	assert.Equal(t, uint64(3), seq3, "Third sequence should be 3")
	assert.True(t, seq3 > seq2 && seq2 > seq1, "Sequence should be monotonic")
}

func TestSequenceManager_Current(t *testing.T) {
	sm := NewSequenceManager()
	
	// Current returns the next sequence number to be assigned
	assert.Equal(t, uint64(1), sm.Current(), "Initial current should be 1")
	
	// After Next(), current should be incremented
	sm.Next()
	assert.Equal(t, uint64(2), sm.Current(), "Current should be 2 after first Next")
}

func TestMessageIDCache_AddAndHas(t *testing.T) {
	cache := NewMessageIDCache()
	
	msgID := "msg-001"
	
	// Should not have initially
	assert.False(t, cache.Has(msgID), "Should not have message initially")
	
	// Add message
	cache.Add(msgID)
	
	// Should have now
	assert.True(t, cache.Has(msgID), "Should have message after adding")
}

func TestSenderSeqTracker_CheckAndUpdate(t *testing.T) {
	tracker := NewSenderSeqTracker()
	sender := blockchain.Address{1, 2, 3}
	
	// First message with seq 1 should be accepted
	assert.True(t, tracker.CheckAndUpdate(sender, 1), "First message should be accepted")
	
	// Duplicate should be rejected
	assert.False(t, tracker.CheckAndUpdate(sender, 1), "Duplicate should be rejected")
	
	// Next in sequence should be accepted
	assert.True(t, tracker.CheckAndUpdate(sender, 2), "Next in sequence should be accepted")
}

func TestVerificationResult(t *testing.T) {
	result := &VerificationResult{
		Valid:        true,
		SenderKnown:  true,
		NotExpired:   true,
		NotDuplicate: true,
	}
	
	assert.True(t, result.Valid)
	assert.True(t, result.SenderKnown)
	assert.True(t, result.NotExpired)
	assert.True(t, result.NotDuplicate)
	assert.Empty(t, result.Error)
}

func TestGroupKeyManager(t *testing.T) {
	manager := NewGroupKeyManager()
	
	platoonID := "platoon-001"
	
	// Generate key
	key1 := manager.GenerateKey(platoonID)
	assert.NotNil(t, key1, "Key should be generated")
	assert.Equal(t, 32, len(key1), "Key should be 32 bytes")
	
	// Get key should return same key
	key2, exists := manager.GetKey(platoonID)
	assert.True(t, exists, "Key should exist")
	assert.Equal(t, key1, key2, "Should get same key")
	
	// Non-existent platoon
	_, exists = manager.GetKey("non-existent")
	assert.False(t, exists, "Non-existent platoon should not have key")
}

func TestGroupKeyManager_EncryptDecrypt(t *testing.T) {
	manager := NewGroupKeyManager()
	platoonID := "platoon-001"
	
	// Generate key
	manager.GenerateKey(platoonID)
	
	plaintext := []byte("secret message")
	
	// Encrypt
	ciphertext, err := manager.EncryptForGroup(platoonID, plaintext)
	assert.NoError(t, err, "Encryption should succeed")
	assert.NotEqual(t, plaintext, ciphertext, "Ciphertext should differ from plaintext")
	
	// Decrypt
	decrypted, err := manager.DecryptForGroup(platoonID, ciphertext)
	assert.NoError(t, err, "Decryption should succeed")
	assert.Equal(t, plaintext, decrypted, "Decrypted should match original")
}
