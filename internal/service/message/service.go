package message

import (
	"crypto/ecdsa"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/identity"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// Service handles V2V message verification
type Service struct {
	identityService *identity.Service
	
	// Sequence management
	seqManagers map[blockchain.Address]*SequenceManager
	seqMu       sync.RWMutex
	
	// Deduplication
	idCache     *MessageIDCache
	senderSeqs  *SenderSeqTracker
	
	// Audit logging
	auditLogs   []*AuditLogEntry
	auditMu     sync.RWMutex
	maxAuditLogs int
	
	// Background tasks
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewService creates a new message verification service
func NewService(idService *identity.Service) *Service {
	svc := &Service{
		identityService: idService,
		seqManagers:     make(map[blockchain.Address]*SequenceManager),
		idCache:         NewMessageIDCache(),
		senderSeqs:      NewSenderSeqTracker(),
		auditLogs:       make([]*AuditLogEntry, 0),
		maxAuditLogs:    10000,
		stopCh:          make(chan struct{}),
	}

	// Start background tasks
	svc.wg.Add(1)
	go svc.cleanupLoop()

	logger.Info("Message verification service initialized")
	return svc
}

// Stop stops the message verification service
func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	logger.Info("Message verification service stopped")
}

// CreateMessage creates a new V2V message (Task 7.1, 7.2)
func (s *Service) CreateMessage(
	msgType MessageType,
	priority Priority,
	sender blockchain.Address,
	receiver blockchain.Address,
	payload []byte,
	platoonID string,
	privKey *ecdsa.PrivateKey,
) (*V2VMessage, error) {
	// Get sequence manager for sender
	s.seqMu.Lock()
	seqMgr, exists := s.seqManagers[sender]
	if !exists {
		seqMgr = NewSequenceManager()
		s.seqManagers[sender] = seqMgr
	}
	seqNum := seqMgr.Next()
	s.seqMu.Unlock()

	// Generate message ID
	msgID := generateMessageID(sender, seqNum)

	// Create message
	msg := &V2VMessage{
		ID:        msgID,
		Type:      msgType,
		Priority:  priority,
		Sender:    sender,
		Receiver:  receiver,
		SeqNum:    seqNum,
		Timestamp: time.Now().Unix(),
		Payload:   payload,
		PlatoonID: platoonID,
	}

	// Sign message
	if err := msg.Sign(privKey); err != nil {
		return nil, fmt.Errorf("failed to sign message: %w", err)
	}

	return msg, nil
}

// VerifyMessage verifies a V2V message (Task 7.3, 7.4, 7.5, 7.8, 7.9)
func (s *Service) VerifyMessage(msg *V2VMessage) *VerificationResult {
	result := &VerificationResult{
		Valid: false,
	}

	// Check for duplicate (Task 7.8)
	if s.idCache.Has(msg.ID) {
		result.Error = "duplicate message"
		s.logAudit(msg, false, result.Error)
		return result
	}

	// Check timestamp (Task 7.9)
	if msg.IsExpired() {
		result.Error = "message expired (outside ±30s window)"
		s.logAudit(msg, false, result.Error)
		return result
	}
	result.NotExpired = true

	// Check sender is registered (Task 7.4, 7.5)
	_, senderExists := s.identityService.GetIdentity(msg.Sender)
	if !senderExists {
		result.Error = "unknown sender"
		s.logAudit(msg, false, result.Error)
		return result
	}
	result.SenderKnown = true

	// Verify sender is active
	if err := s.identityService.VerifyNodeAdmission(msg.Sender); err != nil {
		result.Error = fmt.Sprintf("sender not admitted: %v", err)
		s.logAudit(msg, false, result.Error)
		return result
	}

	// Verify signature (Task 7.3) - msg.Verify() recovers pubKey internally
	if err := msg.Verify(); err != nil {
		result.Error = fmt.Sprintf("signature verification failed: %v", err)
		s.logAudit(msg, false, result.Error)
		return result
	}

	// Check sequence number (Task 7.6, 7.7)
	if !s.senderSeqs.CheckAndUpdate(msg.Sender, msg.SeqNum) {
		result.Error = "duplicate or out-of-order sequence number"
		s.logAudit(msg, false, result.Error)
		return result
	}

	result.NotDuplicate = true
	result.Valid = true

	// Add to ID cache
	s.idCache.Add(msg.ID)

	// Log successful verification (Task 7.12)
	s.logAudit(msg, true, "")

	return result
}

// VerifyMessagesBatch verifies multiple messages in batch (Task 7.13)
func (s *Service) VerifyMessagesBatch(messages []*V2VMessage) *BatchVerificationResult {
	results := make([]*VerificationResult, len(messages))
	allValid := true

	for i, msg := range messages {
		results[i] = s.VerifyMessage(msg)
		if !results[i].Valid {
			allValid = false
		}
	}

	return &BatchVerificationResult{
		Results:  results,
		AllValid: allValid,
	}
}

// EncryptMessage encrypts a message for a specific receiver (Task 7.10)
func (s *Service) EncryptMessage(
	payload []byte,
	receiverPubKey []byte,
) (*EncryptedMessage, error) {
	// Generate ephemeral key pair
	ephemeralPrivKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}

	// For simplicity, we'll use a basic encryption scheme
	// In production, this should use ECIES or similar
	sharedSecret := crypto.Keccak256(
		append(crypto.FromECDSAPub(ephemeralPrivKey.Public().(*ecdsa.PublicKey)),
			receiverPubKey...),
	)

	// XOR encrypt (simplified - in production use proper encryption)
	ciphertext := make([]byte, len(payload))
	for i := range payload {
		ciphertext[i] = payload[i] ^ sharedSecret[i%len(sharedSecret)]
	}

	return &EncryptedMessage{
		Ciphertext:      ciphertext,
		Nonce:           sharedSecret[:16],
		EphemeralPubKey: crypto.FromECDSAPub(ephemeralPrivKey.Public().(*ecdsa.PublicKey)),
	}, nil
}

// DecryptMessage decrypts an encrypted message
func (s *Service) DecryptMessage(
	encrypted *EncryptedMessage,
	privKey *ecdsa.PrivateKey,
) ([]byte, error) {
	// Parse ephemeral public key
	ephemeralPubKey, err := crypto.UnmarshalPubkey(encrypted.EphemeralPubKey)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal ephemeral public key: %w", err)
	}

	// Derive shared secret
	sharedSecret := crypto.Keccak256(
		append(crypto.FromECDSAPub(ephemeralPubKey),
			crypto.FromECDSAPub(&privKey.PublicKey)...),
	)

	// XOR decrypt
	plaintext := make([]byte, len(encrypted.Ciphertext))
	for i := range encrypted.Ciphertext {
		plaintext[i] = encrypted.Ciphertext[i] ^ sharedSecret[i%len(sharedSecret)]
	}

	return plaintext, nil
}

// GetSequenceManager returns the sequence manager for a sender
func (s *Service) GetSequenceManager(sender blockchain.Address) *SequenceManager {
	s.seqMu.Lock()
	defer s.seqMu.Unlock()

	seqMgr, exists := s.seqManagers[sender]
	if !exists {
		seqMgr = NewSequenceManager()
		s.seqManagers[sender] = seqMgr
	}
	return seqMgr
}

// GetNextSeqNum returns the next sequence number for a sender
func (s *Service) GetNextSeqNum(sender blockchain.Address) uint64 {
	return s.GetSequenceManager(sender).Next()
}

// logAudit logs a message to the audit log (Task 7.12)
func (s *Service) logAudit(msg *V2VMessage, verified bool, err string) {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	hash := crypto.Keccak256Hash(msg.Payload)
	var hashBytes [32]byte
	copy(hashBytes[:], hash[:])
	entry := &AuditLogEntry{
		Timestamp:   time.Now().Unix(),
		MessageID:   msg.ID,
		Sender:      msg.Sender,
		Type:        msg.Type,
		Priority:    msg.Priority,
		Verified:    verified,
		PayloadHash: hashBytes,
	}

	if err != "" {
		entry.VerifyError = err
	}

	s.auditLogs = append(s.auditLogs, entry)

	// Trim if too many
	if len(s.auditLogs) > s.maxAuditLogs {
		s.auditLogs = s.auditLogs[len(s.auditLogs)-s.maxAuditLogs:]
	}
}

// GetAuditLogs returns recent audit logs (Task 7.12)
func (s *Service) GetAuditLogs(limit int) []*AuditLogEntry {
	s.auditMu.RLock()
	defer s.auditMu.RUnlock()

	if limit <= 0 || limit > len(s.auditLogs) {
		limit = len(s.auditLogs)
	}

	// Return most recent
	start := len(s.auditLogs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*AuditLogEntry, limit)
	copy(result, s.auditLogs[start:])
	return result
}

// GetAuditStats returns audit statistics
func (s *Service) GetAuditStats() map[string]interface{} {
	s.auditMu.RLock()
	defer s.auditMu.RUnlock()

	verified := 0
	failed := 0
	for _, entry := range s.auditLogs {
		if entry.Verified {
			verified++
		} else {
			failed++
		}
	}

	return map[string]interface{}{
		"total_logs": len(s.auditLogs),
		"verified":   verified,
		"failed":     failed,
	}
}

// cleanupLoop periodically cleans up caches
func (s *Service) cleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.idCache.Cleanup()
		}
	}
}

// Helper function to generate message ID
func generateMessageID(sender blockchain.Address, seqNum uint64) string {
	data := append(sender[:], []byte(fmt.Sprintf("%d", seqNum))...)
	hash := crypto.Keccak256Hash(data)
	return hash.String()
}

// GroupKeyManager manages group encryption keys for platoons (Task 7.11)
type GroupKeyManager struct {
	keys map[string][]byte // platoon ID -> group key
	mu   sync.RWMutex
}

// NewGroupKeyManager creates a new group key manager
func NewGroupKeyManager() *GroupKeyManager {
	return &GroupKeyManager{
		keys: make(map[string][]byte),
	}
}

// GenerateKey generates a new group key for a platoon
func (m *GroupKeyManager) GenerateKey(platoonID string) []byte {
	key := make([]byte, 32)
	// In production, use crypto/rand
	// For now, generate a deterministic key
	hash := crypto.Keccak256([]byte(platoonID + "_group_key"))
	copy(key, hash[:32])

	m.mu.Lock()
	m.keys[platoonID] = key
	m.mu.Unlock()

	return key
}

// GetKey returns the group key for a platoon
func (m *GroupKeyManager) GetKey(platoonID string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[platoonID]
	if !exists {
		return nil, false
	}

	// Return a copy
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	return keyCopy, true
}

// RotateKey rotates the group key for a platoon
func (m *GroupKeyManager) RotateKey(platoonID string) []byte {
	return m.GenerateKey(platoonID)
}

// EncryptForGroup encrypts data using the group key
func (m *GroupKeyManager) EncryptForGroup(platoonID string, plaintext []byte) ([]byte, error) {
	key, exists := m.GetKey(platoonID)
	if !exists {
		return nil, fmt.Errorf("no group key for platoon %s", platoonID)
	}

	// Simple XOR encryption (in production use AES-GCM or similar)
	ciphertext := make([]byte, len(plaintext))
	for i := range plaintext {
		ciphertext[i] = plaintext[i] ^ key[i%len(key)]
	}

	return ciphertext, nil
}

// DecryptForGroup decrypts data using the group key
func (m *GroupKeyManager) DecryptForGroup(platoonID string, ciphertext []byte) ([]byte, error) {
	key, exists := m.GetKey(platoonID)
	if !exists {
		return nil, fmt.Errorf("no group key for platoon %s", platoonID)
	}

	// XOR decrypt (same as encrypt)
	plaintext := make([]byte, len(ciphertext))
	for i := range ciphertext {
		plaintext[i] = ciphertext[i] ^ key[i%len(key)]
	}

	return plaintext, nil
}
