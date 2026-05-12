package consensus

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
)

// NodeState represents the state of a PBFT node
type NodeState uint8

const (
	StateIdle NodeState = iota
	StatePrePrepared
	StatePrepared
	StateCommitted
	StateViewChanging
)

func (s NodeState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StatePrePrepared:
		return "pre-prepared"
	case StatePrepared:
		return "prepared"
	case StateCommitted:
		return "committed"
	case StateViewChanging:
		return "view-changing"
	default:
		return "unknown"
	}
}

// NodeRole represents the role of a node in PBFT
type NodeRole uint8

const (
	RoleUnknown NodeRole = iota
	RolePrimary
	RoleReplica
	RoleFollower // Light client, doesn't participate in consensus
)

func (r NodeRole) String() string {
	switch r {
	case RolePrimary:
		return "primary"
	case RoleReplica:
		return "replica"
	case RoleFollower:
		return "follower"
	default:
		return "unknown"
	}
}

// MessageType represents the type of PBFT message
type MessageType uint8

const (
	MsgRequest MessageType = iota
	MsgPrePrepare
	MsgPrepare
	MsgCommit
	MsgViewChange
	MsgNewView
	MsgCheckpoint
)

func (t MessageType) String() string {
	switch t {
	case MsgRequest:
		return "request"
	case MsgPrePrepare:
		return "pre-prepare"
	case MsgPrepare:
		return "prepare"
	case MsgCommit:
		return "commit"
	case MsgViewChange:
		return "view-change"
	case MsgNewView:
		return "new-view"
	case MsgCheckpoint:
		return "checkpoint"
	default:
		return "unknown"
	}
}

// View represents the current view in PBFT
type View struct {
	Number    uint64             `json:"number"`    // View number (sequence of primary nodes)
	Primary   blockchain.Address `json:"primary"`   // Primary node address
	Timestamp int64              `json:"timestamp"` // View start timestamp
}

// PBFTMessage represents a PBFT protocol message
type PBFTMessage struct {
	Type      MessageType        `json:"type"`
	View      uint64             `json:"view"`
	SeqNumber uint64             `json:"seq_number"` // Sequence number (block height)
	Sender    blockchain.Address `json:"sender"`
	Payload   []byte             `json:"payload"`
	Digest    blockchain.Hash    `json:"digest"`   // Hash of the payload
	Signature []byte             `json:"signature"` // ECDSA signature
	Timestamp int64              `json:"timestamp"`
}

// Sign signs the message with the given private key
func (m *PBFTMessage) Sign(privKey []byte) error {
	// Calculate digest if not set
	if m.Digest == (blockchain.Hash{}) {
		m.Digest = m.CalculateDigest()
	}

	// Serialize message for signing
	data := m.serializeForSign()
	hash := crypto.Keccak256(data)

	privateKey, err := crypto.ToECDSA(privKey)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %w", err)
	}

	sig, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	m.Signature = sig
	return nil
}

// Verify verifies the message signature
func (m *PBFTMessage) Verify(pubKey []byte) error {
	// Verify digest
	calculatedDigest := m.CalculateDigest()
	if !calculatedDigest.Equals(m.Digest) {
		return fmt.Errorf("digest mismatch")
	}

	// Verify signature
	data := m.serializeForSign()
	hash := crypto.Keccak256(data)

	recoveredPubKey, err := crypto.SigToPub(hash, m.Signature)
	if err != nil {
		return fmt.Errorf("failed to recover public key: %w", err)
	}

	recoveredAddr := crypto.PubkeyToAddress(*recoveredPubKey)
	var recoveredBytes [20]byte
	copy(recoveredBytes[:], recoveredAddr[:])
	if recoveredBytes != m.Sender {
		return fmt.Errorf("signature verification failed: sender mismatch")
	}

	return nil
}

// CalculateDigest calculates the message digest
func (m *PBFTMessage) CalculateDigest() blockchain.Hash {
	data := m.serializeForSign()
	hash := crypto.Keccak256Hash(data)
	var result [32]byte
	copy(result[:], hash[:])
	return result
}

func (m *PBFTMessage) serializeForSign() []byte {
	// Exclude signature from serialization
	data, _ := json.Marshal(struct {
		Type      MessageType        `json:"type"`
		View      uint64             `json:"view"`
		SeqNumber uint64             `json:"seq_number"`
		Sender    blockchain.Address `json:"sender"`
		Payload   []byte             `json:"payload"`
		Digest    blockchain.Hash    `json:"digest"`
		Timestamp int64              `json:"timestamp"`
	}{
		Type:      m.Type,
		View:      m.View,
		SeqNumber: m.SeqNumber,
		Sender:    m.Sender,
		Payload:   m.Payload,
		Digest:    m.Digest,
		Timestamp: m.Timestamp,
	})
	return data
}

// PrePreparePayload represents the payload of a Pre-Prepare message
type PrePreparePayload struct {
	BlockHash   blockchain.Hash    `json:"block_hash"`
	BlockData   *blockchain.Block  `json:"block_data"`
	Validator   blockchain.Address `json:"validator"`
}

// PreparePayload represents the payload of a Prepare message
type PreparePayload struct {
	BlockHash blockchain.Hash `json:"block_hash"`
}

// CommitPayload represents the payload of a Commit message
type CommitPayload struct {
	BlockHash blockchain.Hash `json:"block_hash"`
}

// ViewChangePayload represents the payload of a View-Change message
type ViewChangePayload struct {
	NewViewNumber uint64              `json:"new_view_number"`
	LastStableCheckpoint uint64       `json:"last_stable_checkpoint"`
	PreparedProofs       []Proof      `json:"prepared_proofs"` // Proofs of prepared blocks
}

// Proof represents a proof of prepared state
type Proof struct {
	View      uint64          `json:"view"`
	SeqNumber uint64          `json:"seq_number"`
	BlockHash blockchain.Hash `json:"block_hash"`
	Signatures [][]byte       `json:"signatures"` // Quorum of signatures
}

// NewViewPayload represents the payload of a New-View message
type NewViewPayload struct {
	NewViewNumber uint64              `json:"new_view_number"`
	ViewChangeProofs     []Proof      `json:"view_change_proofs"`
	PrePrepareMsgs       []*PBFTMessage `json:"pre_prepare_msgs"`
}

// CheckpointPayload represents the payload of a Checkpoint message
type CheckpointPayload struct {
	SeqNumber uint64          `json:"seq_number"`
	BlockHash blockchain.Hash `json:"block_hash"`
}

// ValidatorSet represents the set of validators
type ValidatorSet struct {
	Validators  []blockchain.Address `json:"validators"`
	PrimaryIdx  int                  `json:"primary_idx"`
	ViewNumber  uint64               `json:"view_number"`
}

// Size returns the number of validators
func (vs *ValidatorSet) Size() int {
	return len(vs.Validators)
}

// Quorum returns the minimum number of messages needed for consensus (2f+1)
func (vs *ValidatorSet) Quorum() int {
	return (len(vs.Validators) * 2 / 3) + 1
}

// FaultTolerance returns the maximum number of faulty nodes (f)
func (vs *ValidatorSet) FaultTolerance() int {
	return (len(vs.Validators) - 1) / 3
}

// GetPrimary returns the primary validator for the current view
func (vs *ValidatorSet) GetPrimary() blockchain.Address {
	if len(vs.Validators) == 0 {
		return blockchain.Address{}
	}
	idx := int(vs.ViewNumber) % len(vs.Validators)
	return vs.Validators[idx]
}

// IsValidator checks if an address is a validator
func (vs *ValidatorSet) IsValidator(addr blockchain.Address) bool {
	for _, v := range vs.Validators {
		if v == addr {
			return true
		}
	}
	return false
}

// UpdateView updates the view number and primary index
func (vs *ValidatorSet) UpdateView(viewNumber uint64) {
	vs.ViewNumber = viewNumber
}

// AddValidator adds a new validator (requires consensus)
func (vs *ValidatorSet) AddValidator(addr blockchain.Address) error {
	if vs.IsValidator(addr) {
		return fmt.Errorf("validator already exists")
	}
	vs.Validators = append(vs.Validators, addr)
	return nil
}

// RemoveValidator removes a validator (requires consensus)
func (vs *ValidatorSet) RemoveValidator(addr blockchain.Address) error {
	for i, v := range vs.Validators {
		if v == addr {
			// Remove by swapping with last and truncating
			vs.Validators[i] = vs.Validators[len(vs.Validators)-1]
			vs.Validators = vs.Validators[:len(vs.Validators)-1]
			return nil
		}
	}
	return fmt.Errorf("validator not found")
}

// ConsensusState represents the internal state of consensus for a sequence number
type ConsensusState struct {
	View           uint64
	SeqNumber      uint64
	BlockHash      blockchain.Hash
	PrePrepareMsg  *PBFTMessage
	PrepareMsgs    map[blockchain.Address]*PBFTMessage
	CommitMsgs     map[blockchain.Address]*PBFTMessage
	State          NodeState
	PreparedProof  *Proof
	CommittedProof *Proof
}

// NewConsensusState creates a new consensus state
func NewConsensusState(view, seqNum uint64) *ConsensusState {
	return &ConsensusState{
		View:        view,
		SeqNumber:   seqNum,
		PrepareMsgs: make(map[blockchain.Address]*PBFTMessage),
		CommitMsgs:  make(map[blockchain.Address]*PBFTMessage),
		State:       StateIdle,
	}
}

// AddPrepareMsg adds a prepare message
func (cs *ConsensusState) AddPrepareMsg(msg *PBFTMessage) {
	cs.PrepareMsgs[msg.Sender] = msg
}

// AddCommitMsg adds a commit message
func (cs *ConsensusState) AddCommitMsg(msg *PBFTMessage) {
	cs.CommitMsgs[msg.Sender] = msg
}

// HasPrepared checks if prepared state is reached
func (cs *ConsensusState) HasPrepared(quorum int) bool {
	// Pre-prepare + (quorum - 1) prepares = quorum
	if cs.PrePrepareMsg == nil {
		return false
	}
	return len(cs.PrepareMsgs) >= quorum-1
}

// HasCommitted checks if committed state is reached
func (cs *ConsensusState) HasCommitted(quorum int) bool {
	return len(cs.CommitMsgs) >= quorum
}

// GetPrepareCount returns the number of prepare messages
func (cs *ConsensusState) GetPrepareCount() int {
	return len(cs.PrepareMsgs)
}

// GetCommitCount returns the number of commit messages
func (cs *ConsensusState) GetCommitCount() int {
	return len(cs.CommitMsgs)
}

// RequestBatch represents a batch of client requests
type RequestBatch struct {
	Transactions []*blockchain.Transaction `json:"transactions"`
	Timestamp    int64                     `json:"timestamp"`
	BatchHash    blockchain.Hash           `json:"batch_hash"`
}

// CalculateHash calculates the hash of the batch
func (rb *RequestBatch) CalculateHash() blockchain.Hash {
	data, _ := json.Marshal(rb.Transactions)
	hash := crypto.Keccak256Hash(data)
	var result [32]byte
	copy(result[:], hash[:])
	return result
}

// Checkpoint represents a stable checkpoint
type Checkpoint struct {
	SeqNumber uint64          `json:"seq_number"`
	BlockHash blockchain.Hash `json:"block_hash"`
	Proof     *Proof          `json:"proof"`
	Timestamp int64           `json:"timestamp"`
}

// ViewChangeState tracks view change state
type ViewChangeState struct {
	NewViewNumber uint64
	ViewChanges   map[blockchain.Address]*PBFTMessage
	StartedAt     int64
}

// NewViewChangeState creates a new view change state
func NewViewChangeState(newView uint64) *ViewChangeState {
	return &ViewChangeState{
		NewViewNumber: newView,
		ViewChanges:   make(map[blockchain.Address]*PBFTMessage),
	}
}

// AddViewChange adds a view change message
func (vcs *ViewChangeState) AddViewChange(msg *PBFTMessage) {
	vcs.ViewChanges[msg.Sender] = msg
}

// HasQuorum checks if enough view change messages received
func (vcs *ViewChangeState) HasQuorum(quorum int) bool {
	return len(vcs.ViewChanges) >= quorum
}

// IsValidValidator checks if sender is in validator set
func (vcs *ViewChangeState) IsValidValidator(sender blockchain.Address, validators *ValidatorSet) bool {
	return validators.IsValidator(sender)
}
