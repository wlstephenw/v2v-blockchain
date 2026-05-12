package message

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
)

// MessageType represents the type of V2V message
type MessageType uint8

const (
	MsgTypeHeartbeat MessageType = iota
	MsgTypeControl
	MsgTypeEmergency
	MsgTypeStatus
	MsgTypePlatoonControl
	MsgTypeJoinRequest
	MsgTypeJoinResponse
)

func (t MessageType) String() string {
	switch t {
	case MsgTypeHeartbeat:
		return "heartbeat"
	case MsgTypeControl:
		return "control"
	case MsgTypeEmergency:
		return "emergency"
	case MsgTypeStatus:
		return "status"
	case MsgTypePlatoonControl:
		return "platoon_control"
	case MsgTypeJoinRequest:
		return "join_request"
	case MsgTypeJoinResponse:
		return "join_response"
	default:
		return "unknown"
	}
}

// Priority represents message priority
type Priority uint8

const (
	PriorityLow Priority = iota
	PriorityNormal
	PriorityHigh
	PriorityEmergency
)

// V2VMessage represents a V2V message
type V2VMessage struct {
	ID        string             `json:"id"`        // Unique message ID
	Type      MessageType        `json:"type"`      // Message type
	Priority  Priority           `json:"priority"`  // Message priority
	Sender    blockchain.Address `json:"sender"`    // Sender address
	Receiver  blockchain.Address `json:"receiver"`  // Receiver address (optional, broadcast if empty)
	SeqNum    uint64             `json:"seq_num"`   // Sequence number for ordering
	Timestamp int64              `json:"timestamp"` // Unix timestamp
	Payload   []byte             `json:"payload"`   // Message payload
	Signature []byte             `json:"signature"` // ECDSA signature (65 bytes)
	PlatoonID string             `json:"platoon_id,omitempty"` // Associated platoon ID
}

// CalculateHash calculates the hash of the message (for signing)
func (m *V2VMessage) CalculateHash() []byte {
	// Hash everything except signature
	data, _ := json.Marshal(struct {
		ID        string             `json:"id"`
		Type      MessageType        `json:"type"`
		Priority  Priority           `json:"priority"`
		Sender    blockchain.Address `json:"sender"`
		Receiver  blockchain.Address `json:"receiver"`
		SeqNum    uint64             `json:"seq_num"`
		Timestamp int64              `json:"timestamp"`
		Payload   []byte             `json:"payload"`
		PlatoonID string             `json:"platoon_id,omitempty"`
	}{
		ID:        m.ID,
		Type:      m.Type,
		Priority:  m.Priority,
		Sender:    m.Sender,
		Receiver:  m.Receiver,
		SeqNum:    m.SeqNum,
		Timestamp: m.Timestamp,
		Payload:   m.Payload,
		PlatoonID: m.PlatoonID,
	})

	hash := crypto.Keccak256Hash(data)
	return hash.Bytes()
}

// Sign signs the message with the given private key
func (m *V2VMessage) Sign(privKey *ecdsa.PrivateKey) error {
	// Calculate hash
	hash := m.CalculateHash()

	// Sign
	sig, err := crypto.Sign(hash, privKey)
	if err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	m.Signature = sig
	return nil
}

// Verify verifies the message signature
func (m *V2VMessage) Verify() error {
	if len(m.Signature) == 0 {
		return fmt.Errorf("missing signature")
	}

	if len(m.Signature) != 65 {
		return fmt.Errorf("invalid signature length: expected 65, got %d", len(m.Signature))
	}

	hash := m.CalculateHash()

	// Recover public key from signature
	pubKey, err := crypto.SigToPub(hash, m.Signature)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	// Verify sender matches recovered address
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)
	var recoveredBytes [20]byte
	copy(recoveredBytes[:], recoveredAddr[:])
	if recoveredBytes != m.Sender {
		return fmt.Errorf("signature verification failed: sender mismatch")
	}

	return nil
}

// IsExpired checks if the message is expired (±30 seconds window)
func (m *V2VMessage) IsExpired() bool {
	now := time.Now().Unix()
	diff := now - m.Timestamp
	return diff > 30 || diff < -30
}

// IsBroadcast checks if this is a broadcast message
func (m *V2VMessage) IsBroadcast() bool {
	return m.Receiver == blockchain.Address{}
}

// SequenceManager manages sequence numbers for a sender
type SequenceManager struct {
	nextSeqNum uint64
}

// NewSequenceManager creates a new sequence manager
func NewSequenceManager() *SequenceManager {
	return &SequenceManager{
		nextSeqNum: 1,
	}
}

// Next returns the next sequence number (atomic)
func (sm *SequenceManager) Next() uint64 {
	return atomic.AddUint64(&sm.nextSeqNum, 1) - 1
}

// Current returns the current sequence number
func (sm *SequenceManager) Current() uint64 {
	return atomic.LoadUint64(&sm.nextSeqNum)
}

// Set sets the sequence number (for recovery)
func (sm *SequenceManager) Set(seqNum uint64) {
	atomic.StoreUint64(&sm.nextSeqNum, seqNum+1)
}

// MessageIDCache tracks recently seen message IDs for deduplication
type MessageIDCache struct {
	ids    map[string]int64 // message ID -> timestamp
	maxAge int64            // maximum age in seconds
}

// NewMessageIDCache creates a new message ID cache
func NewMessageIDCache() *MessageIDCache {
	return &MessageIDCache{
		ids:    make(map[string]int64),
		maxAge: 600, // 10 minutes default
	}
}

// Has checks if a message ID has been seen
func (c *MessageIDCache) Has(id string) bool {
	_, exists := c.ids[id]
	return exists
}

// Add adds a message ID to the cache
func (c *MessageIDCache) Add(id string) {
	c.ids[id] = time.Now().Unix()
}

// Cleanup removes old entries from the cache
func (c *MessageIDCache) Cleanup() {
	now := time.Now().Unix()
	for id, timestamp := range c.ids {
		if now-timestamp > c.maxAge {
			delete(c.ids, id)
		}
	}
}

// Size returns the current size of the cache
func (c *MessageIDCache) Size() int {
	return len(c.ids)
}

// SenderSeqTracker tracks sequence numbers per sender
type SenderSeqTracker struct {
	sequences map[blockchain.Address]uint64
}

// NewSenderSeqTracker creates a new sender sequence tracker
func NewSenderSeqTracker() *SenderSeqTracker {
	return &SenderSeqTracker{
		sequences: make(map[blockchain.Address]uint64),
	}
}

// CheckAndUpdate checks if sequence number is valid and updates tracker
func (t *SenderSeqTracker) CheckAndUpdate(sender blockchain.Address, seqNum uint64) bool {
	lastSeq, exists := t.sequences[sender]
	if !exists {
		// First message from this sender
		t.sequences[sender] = seqNum
		return true
	}

	if seqNum <= lastSeq {
		// Duplicate or out of order
		return false
	}

	t.sequences[sender] = seqNum
	return true
}

// GetLastSeq returns the last seen sequence number for a sender
func (t *SenderSeqTracker) GetLastSeq(sender blockchain.Address) (uint64, bool) {
	seq, exists := t.sequences[sender]
	return seq, exists
}

// EncryptedMessage represents an encrypted message payload
type EncryptedMessage struct {
	Ciphertext []byte `json:"ciphertext"` // Encrypted payload
	Nonce      []byte `json:"nonce"`      // Encryption nonce
	EphemeralPubKey []byte `json:"ephemeral_pub_key"` // Ephemeral public key (for ECIES)
}

// AuditLogEntry represents a message audit log entry
type AuditLogEntry struct {
	Timestamp   int64              `json:"timestamp"`
	MessageID   string             `json:"message_id"`
	Sender      blockchain.Address `json:"sender"`
	Type        MessageType        `json:"type"`
	Priority    Priority           `json:"priority"`
	Verified    bool               `json:"verified"`
	VerifyError string             `json:"verify_error,omitempty"`
	PayloadHash blockchain.Hash    `json:"payload_hash"`
}

// VerificationResult represents the result of message verification
type VerificationResult struct {
	Valid       bool   `json:"valid"`
	Error       string `json:"error,omitempty"`
	SenderKnown bool   `json:"sender_known"`
	NotExpired  bool   `json:"not_expired"`
	NotDuplicate bool  `json:"not_duplicate"`
}

// BatchVerificationRequest represents a batch of messages to verify
type BatchVerificationRequest struct {
	Messages []*V2VMessage `json:"messages"`
}

// BatchVerificationResult represents the result of batch verification
type BatchVerificationResult struct {
	Results []*VerificationResult `json:"results"`
	AllValid bool                 `json:"all_valid"`
}
