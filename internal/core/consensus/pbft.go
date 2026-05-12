package consensus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/infra/storage"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// BroadcastFunc is the function type for broadcasting messages to the network
type BroadcastFunc func([]byte) error

// PBFTEngine implements the Practical Byzantine Fault Tolerance consensus algorithm
type PBFTEngine struct {
	// Node identity
	address    blockchain.Address
	privateKey []byte
	role       NodeRole

	// Blockchain
	bc         *blockchain.Blockchain
	storage    storage.Storage

	// Validator set
	validators *ValidatorSet
	valSetMu   sync.RWMutex

	// Current view
	view       *View
	viewMu     sync.RWMutex

	// Consensus states by sequence number
	states     map[uint64]*ConsensusState
	statesMu   sync.RWMutex

	// Checkpoints
	checkpoints   map[uint64]*Checkpoint
	lastCheckpoint uint64
	checkpointMu   sync.RWMutex

	// View change state
	viewChangeState *ViewChangeState
	vcMu            sync.Mutex

	// Request pool
	requestPool     []*blockchain.Transaction
	requestPoolMu   sync.Mutex
	highPriorityPool []*blockchain.Transaction

	// Message handlers
	messageHandler  func(*PBFTMessage) error
	blockCommitHandler func(*blockchain.Block) error
	broadcastFunc   BroadcastFunc

	// Timers
	viewChangeTimer  *time.Timer
	requestTimer     *time.Timer
	checkpointTimer  *time.Timer

	// Configuration
	config         *PBFTConfig

	// Metrics
	metrics        *ConsensusMetrics

	// Lifecycle
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	running        bool
}

// PBFTConfig holds PBFT configuration
type PBFTConfig struct {
	ViewTimeout        time.Duration
	RequestTimeout     time.Duration
	CheckpointInterval uint64
	BlockInterval      time.Duration
	MaxTxPerBlock      int
}

// DefaultPBFTConfig returns default PBFT configuration
func DefaultPBFTConfig() *PBFTConfig {
	return &PBFTConfig{
		ViewTimeout:        10 * time.Second,
		RequestTimeout:     5 * time.Second,
		CheckpointInterval: 100,
		BlockInterval:      2 * time.Second,
		MaxTxPerBlock:      100,
	}
}

// ConsensusMetrics tracks consensus metrics
type ConsensusMetrics struct {
	ViewChanges      uint64
	BlocksCommitted  uint64
	ViewChangeTime   time.Duration
	ConsensusTime    time.Duration
	mu               sync.RWMutex
}

// NewPBFTEngine creates a new PBFT consensus engine
func NewPBFTEngine(
	address blockchain.Address,
	privKey []byte,
	bc *blockchain.Blockchain,
	store storage.Storage,
	validators []blockchain.Address,
	config *PBFTConfig,
) (*PBFTEngine, error) {
	if config == nil {
		config = DefaultPBFTConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &PBFTEngine{
		address:        address,
		privateKey:     privKey,
		bc:             bc,
		storage:        store,
		validators:     &ValidatorSet{Validators: validators, ViewNumber: 0},
		view:           &View{Number: 0, Primary: validators[0]},
		states:         make(map[uint64]*ConsensusState),
		checkpoints:    make(map[uint64]*Checkpoint),
		requestPool:    make([]*blockchain.Transaction, 0),
		highPriorityPool: make([]*blockchain.Transaction, 0),
		config:         config,
		metrics:        &ConsensusMetrics{},
		ctx:            ctx,
		cancel:         cancel,
	}

	// Determine initial role
	if engine.view.Primary == address {
		engine.role = RolePrimary
	} else if engine.validators.IsValidator(address) {
		engine.role = RoleReplica
	} else {
		engine.role = RoleFollower
	}

	logger.Info("PBFT engine created",
		logger.String("address", address.String()),
		logger.String("role", engine.role.String()),
		logger.Int("validators", len(validators)),
	)

	return engine, nil
}

// Start starts the PBFT engine
func (e *PBFTEngine) Start() error {
	e.wg.Add(1)
	go e.mainLoop()

	logger.Info("PBFT engine started",
		logger.String("role", e.role.String()),
		logger.Uint64("view", e.view.Number),
	)

	return nil
}

// Stop stops the PBFT engine
func (e *PBFTEngine) Stop() {
	e.cancel()
	e.wg.Wait()
	logger.Info("PBFT engine stopped")
}

// mainLoop is the main consensus loop
func (e *PBFTEngine) mainLoop() {
	defer e.wg.Done()

	// Start block production timer for primary
	if e.role == RolePrimary {
		e.startBlockTimer()
	}

	// Start view change timer
	e.startViewChangeTimer()

	for {
		select {
		case <-e.ctx.Done():
			return
		}
	}
}

// SetMessageHandler sets the message handler for sending PBFT messages
func (e *PBFTEngine) SetMessageHandler(handler func(*PBFTMessage) error) {
	e.messageHandler = handler
}

// SetBlockCommitHandler sets the handler for committed blocks
func (e *PBFTEngine) SetBlockCommitHandler(handler func(*blockchain.Block) error) {
	e.blockCommitHandler = handler
}

// SetBroadcastFunc sets the broadcast function for sending messages to the network
func (e *PBFTEngine) SetBroadcastFunc(fn BroadcastFunc) {
	e.broadcastFunc = fn
	logger.Info("PBFT broadcast function registered")
}

// broadcastToNetwork broadcasts a message to the network using the broadcast function
func (e *PBFTEngine) broadcastToNetwork(msg *PBFTMessage) error {
	if e.broadcastFunc == nil {
		return errors.New("broadcast function not set")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	if err := e.broadcastFunc(data); err != nil {
		return fmt.Errorf("failed to broadcast message: %w", err)
	}

	return nil
}

// SubmitTransaction submits a transaction to the consensus pool
func (e *PBFTEngine) SubmitTransaction(tx *blockchain.Transaction, highPriority bool) error {
	e.requestPoolMu.Lock()
	defer e.requestPoolMu.Unlock()

	// Check if already in pool
	for _, existing := range e.requestPool {
		if existing.Hash == tx.Hash {
			return errors.New("transaction already in pool")
		}
	}

	if highPriority {
		e.highPriorityPool = append(e.highPriorityPool, tx)
	} else {
		e.requestPool = append(e.requestPool, tx)
	}

	return nil
}

// GetPendingTransactions returns pending transactions from the pool
func (e *PBFTEngine) GetPendingTransactions(max int) []*blockchain.Transaction {
	e.requestPoolMu.Lock()
	defer e.requestPoolMu.Unlock()

	// First include high priority transactions
	txs := make([]*blockchain.Transaction, 0, max)

	// Add high priority first
	for i := 0; i < len(e.highPriorityPool) && len(txs) < max; i++ {
		txs = append(txs, e.highPriorityPool[i])
	}
	// Remove included high priority txs
	if len(txs) > 0 {
		e.highPriorityPool = e.highPriorityPool[len(txs):]
	}

	// Add regular transactions
	remaining := max - len(txs)
	for i := 0; i < len(e.requestPool) && i < remaining; i++ {
		txs = append(txs, e.requestPool[i])
	}
	// Remove included regular txs
	if remaining > 0 && len(e.requestPool) > 0 {
		count := remaining
		if count > len(e.requestPool) {
			count = len(e.requestPool)
		}
		e.requestPool = e.requestPool[count:]
	}

	return txs
}

// HandleMessage handles incoming PBFT messages
func (e *PBFTEngine) HandleMessage(msg *PBFTMessage) error {
	// Verify message
	if err := msg.Verify(nil); err != nil {
		return fmt.Errorf("message verification failed: %w", err)
	}

	// Verify sender is a validator
	if !e.validators.IsValidator(msg.Sender) {
		return errors.New("sender is not a validator")
	}

	switch msg.Type {
	case MsgPrePrepare:
		return e.handlePrePrepare(msg)
	case MsgPrepare:
		return e.handlePrepare(msg)
	case MsgCommit:
		return e.handleCommit(msg)
	case MsgViewChange:
		return e.handleViewChange(msg)
	case MsgNewView:
		return e.handleNewView(msg)
	default:
		return fmt.Errorf("unknown message type: %d", msg.Type)
	}
}

// handlePrePrepare handles Pre-Prepare messages (Task 5.2.2)
func (e *PBFTEngine) handlePrePrepare(msg *PBFTMessage) error {
	e.viewMu.RLock()
	currentView := e.view.Number
	e.viewMu.RUnlock()

	// Verify view matches
	if msg.View != currentView {
		return fmt.Errorf("view mismatch: expected %d, got %d", currentView, msg.View)
	}

	// Verify sender is primary
	primary := e.validators.GetPrimary()
	if msg.Sender != primary {
		return errors.New("pre-prepare not from primary")
	}

	// Deserialize payload
	var payload PrePreparePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Verify block
	block := payload.BlockData
	if block == nil {
		return errors.New("block data is nil")
	}

	// Get or create consensus state
	e.statesMu.Lock()
	state, exists := e.states[msg.SeqNumber]
	if !exists {
		state = NewConsensusState(msg.View, msg.SeqNumber)
		e.states[msg.SeqNumber] = state
	}
	state.PrePrepareMsg = msg
	state.BlockHash = payload.BlockHash
	e.statesMu.Unlock()

	logger.Debug("Received Pre-Prepare",
		logger.Uint64("seq", msg.SeqNumber),
		logger.String("hash", payload.BlockHash.String()),
	)

	// Broadcast Prepare message
	if e.role == RoleReplica || e.role == RolePrimary {
		return e.broadcastPrepare(msg.SeqNumber, payload.BlockHash)
	}

	return nil
}

// broadcastPrepare broadcasts a Prepare message (Task 5.2.2)
func (e *PBFTEngine) broadcastPrepare(seqNum uint64, blockHash blockchain.Hash) error {
	payload := PreparePayload{BlockHash: blockHash}
	payloadBytes, _ := json.Marshal(payload)

	msg := &PBFTMessage{
		Type:      MsgPrepare,
		View:      e.view.Number,
		SeqNumber: seqNum,
		Sender:    e.address,
		Payload:   payloadBytes,
		Timestamp: time.Now().Unix(),
	}

	if err := msg.Sign(e.privateKey); err != nil {
		return fmt.Errorf("failed to sign prepare: %w", err)
	}

	// Broadcast to network
	if err := e.broadcastToNetwork(msg); err != nil {
		logger.Debug("Failed to broadcast prepare to network", logger.ErrField(err))
	}

	// Also call local message handler for processing
	if e.messageHandler != nil {
		if err := e.messageHandler(msg); err != nil {
			return fmt.Errorf("failed to handle prepare locally: %w", err)
		}
	}

	return nil
}

// handlePrepare handles Prepare messages (Task 5.2.3)
func (e *PBFTEngine) handlePrepare(msg *PBFTMessage) error {
	e.statesMu.Lock()
	defer e.statesMu.Unlock()

	state, exists := e.states[msg.SeqNumber]
	if !exists {
		state = NewConsensusState(msg.View, msg.SeqNumber)
		e.states[msg.SeqNumber] = state
	}

	// Don't accept prepares without pre-prepare
	if state.PrePrepareMsg == nil {
		return errors.New("no pre-prepare received for this sequence")
	}

	// Deserialize payload
	var payload PreparePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal prepare payload: %w", err)
	}

	// Verify block hash matches
	if !payload.BlockHash.Equals(state.BlockHash) {
		return errors.New("block hash mismatch in prepare")
	}

	// Add prepare message
	state.AddPrepareMsg(msg)

	logger.Debug("Received Prepare",
		logger.Uint64("seq", msg.SeqNumber),
		logger.String("sender", msg.Sender.String()),
		logger.Int("prepare_count", state.GetPrepareCount()),
	)

	// Check if prepared state reached
	quorum := e.validators.Quorum()
	if state.State < StatePrepared && state.HasPrepared(quorum) {
		state.State = StatePrepared
		logger.Info("Reached Prepared state",
			logger.Uint64("seq", msg.SeqNumber),
			logger.Int("quorum", quorum),
		)

		// Broadcast Commit message
		return e.broadcastCommit(msg.SeqNumber, state.BlockHash)
	}

	return nil
}

// broadcastCommit broadcasts a Commit message (Task 5.2.4)
func (e *PBFTEngine) broadcastCommit(seqNum uint64, blockHash blockchain.Hash) error {
	payload := CommitPayload{BlockHash: blockHash}
	payloadBytes, _ := json.Marshal(payload)

	msg := &PBFTMessage{
		Type:      MsgCommit,
		View:      e.view.Number,
		SeqNumber: seqNum,
		Sender:    e.address,
		Payload:   payloadBytes,
		Timestamp: time.Now().Unix(),
	}

	if err := msg.Sign(e.privateKey); err != nil {
		return fmt.Errorf("failed to sign commit: %w", err)
	}

	// Broadcast to network
	if err := e.broadcastToNetwork(msg); err != nil {
		logger.Debug("Failed to broadcast commit to network", logger.ErrField(err))
	}

	// Also call local message handler for processing
	if e.messageHandler != nil {
		if err := e.messageHandler(msg); err != nil {
			return fmt.Errorf("failed to handle commit locally: %w", err)
		}
	}

	return nil
}

// handleCommit handles Commit messages (Task 5.2.5)
func (e *PBFTEngine) handleCommit(msg *PBFTMessage) error {
	e.statesMu.Lock()
	defer e.statesMu.Unlock()

	state, exists := e.states[msg.SeqNumber]
	if !exists {
		return errors.New("no consensus state for this sequence")
	}

	// Deserialize payload
	var payload CommitPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal commit payload: %w", err)
	}

	// Verify block hash matches
	if !payload.BlockHash.Equals(state.BlockHash) {
		return errors.New("block hash mismatch in commit")
	}

	// Add commit message
	state.AddCommitMsg(msg)

	logger.Debug("Received Commit",
		logger.Uint64("seq", msg.SeqNumber),
		logger.String("sender", msg.Sender.String()),
		logger.Int("commit_count", state.GetCommitCount()),
	)

	// Check if committed state reached (Task 5.2.6)
	quorum := e.validators.Quorum()
	if state.State < StateCommitted && state.HasCommitted(quorum) {
		state.State = StateCommitted
		return e.commitBlock(msg.SeqNumber)
	}

	return nil
}

// commitBlock commits a block to the blockchain (Task 5.2.6)
func (e *PBFTEngine) commitBlock(seqNum uint64) error {
	e.statesMu.RLock()
	state := e.states[seqNum]
	e.statesMu.RUnlock()

	if state == nil || state.PrePrepareMsg == nil {
		return errors.New("cannot commit without pre-prepare")
	}

	var payload PrePreparePayload
	if err := json.Unmarshal(state.PrePrepareMsg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal pre-prepare payload: %w", err)
	}

	block := payload.BlockData

	// Add block to blockchain
	if err := e.bc.AddBlock(block); err != nil {
		return fmt.Errorf("failed to add block to chain: %w", err)
	}

	// Call block commit handler
	if e.blockCommitHandler != nil {
		if err := e.blockCommitHandler(block); err != nil {
			logger.Warn("Block commit handler failed", logger.ErrField(err))
		}
	}

	// Update metrics
	e.metrics.mu.Lock()
	e.metrics.BlocksCommitted++
	e.metrics.mu.Unlock()

	logger.Info("Block committed",
		logger.Uint64("height", block.Header.Height),
		logger.String("hash", block.Header.Hash.String()),
		logger.Int("tx_count", len(block.Transactions)),
	)

	return nil
}

// ProposeBlock proposes a new block (called by primary) (Task 5.2.1)
func (e *PBFTEngine) ProposeBlock() error {
	if e.role != RolePrimary {
		return errors.New("only primary can propose blocks")
	}

	// Get pending transactions
	txs := e.GetPendingTransactions(e.config.MaxTxPerBlock)
	if len(txs) == 0 {
		return errors.New("no pending transactions")
	}

	// Create block
	latestHeight := e.bc.GetLatestHeight()
	block, err := e.bc.CreateBlock(txs, e.address)
	if err != nil {
		return fmt.Errorf("failed to create block: %w", err)
	}

	// Sign block
	if err := block.Sign(e.privateKey); err != nil {
		return fmt.Errorf("failed to sign block: %w", err)
	}

	// Create Pre-Prepare message
	payload := PrePreparePayload{
		BlockHash: block.Header.Hash,
		BlockData: block,
		Validator: e.address,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &PBFTMessage{
		Type:      MsgPrePrepare,
		View:      e.view.Number,
		SeqNumber: latestHeight + 1,
		Sender:    e.address,
		Payload:   payloadBytes,
		Timestamp: time.Now().Unix(),
	}

	if err := msg.Sign(e.privateKey); err != nil {
		return fmt.Errorf("failed to sign pre-prepare: %w", err)
	}

	// Broadcast to network
	if err := e.broadcastToNetwork(msg); err != nil {
		logger.Debug("Failed to broadcast pre-prepare to network", logger.ErrField(err))
	}

	// Also call local message handler for processing
	if e.messageHandler != nil {
		if err := e.messageHandler(msg); err != nil {
			return fmt.Errorf("failed to handle pre-prepare locally: %w", err)
		}
	}

	logger.Info("Proposed block",
		logger.Uint64("height", block.Header.Height),
		logger.String("hash", block.Header.Hash.String()),
		logger.Int("tx_count", len(txs)),
	)

	return nil
}

// handleViewChange handles View-Change messages (Task 5.3.2)
func (e *PBFTEngine) handleViewChange(msg *PBFTMessage) error {
	e.vcMu.Lock()
	defer e.vcMu.Unlock()

	var payload ViewChangePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal view change payload: %w", err)
	}

	// Initialize view change state if needed
	if e.viewChangeState == nil || e.viewChangeState.NewViewNumber != payload.NewViewNumber {
		e.viewChangeState = NewViewChangeState(payload.NewViewNumber)
	}

	// Add view change message
	e.viewChangeState.AddViewChange(msg)

	logger.Info("Received View-Change",
		logger.Uint64("new_view", payload.NewViewNumber),
		logger.String("sender", msg.Sender.String()),
		logger.Int("count", len(e.viewChangeState.ViewChanges)),
	)

	// Check if we have enough view changes to trigger new view
	quorum := e.validators.Quorum()
	if e.viewChangeState.HasQuorum(quorum) {
		// If we are the new primary, broadcast New-View
		e.validators.UpdateView(payload.NewViewNumber)
		newPrimary := e.validators.GetPrimary()

		if newPrimary == e.address {
			return e.broadcastNewView(e.viewChangeState)
		}
	}

	return nil
}

// broadcastNewView broadcasts a New-View message (Task 5.3.5)
func (e *PBFTEngine) broadcastNewView(vcState *ViewChangeState) error {
	// Collect view change proofs
	proofs := make([]Proof, 0, len(vcState.ViewChanges))
	for _, msg := range vcState.ViewChanges {
		proof := Proof{
			View:      msg.View,
			SeqNumber: msg.SeqNumber,
			Signatures: [][]byte{msg.Signature},
		}
		proofs = append(proofs, proof)
	}

	// Create pre-prepare messages for pending requests
	prePrepares := make([]*PBFTMessage, 0)

	payload := NewViewPayload{
		NewViewNumber:    vcState.NewViewNumber,
		ViewChangeProofs: proofs,
		PrePrepareMsgs:   prePrepares,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &PBFTMessage{
		Type:      MsgNewView,
		View:      vcState.NewViewNumber,
		Sender:    e.address,
		Payload:   payloadBytes,
		Timestamp: time.Now().Unix(),
	}

	if err := msg.Sign(e.privateKey); err != nil {
		return fmt.Errorf("failed to sign new-view: %w", err)
	}

	// Broadcast to network
	if err := e.broadcastToNetwork(msg); err != nil {
		logger.Debug("Failed to broadcast new-view to network", logger.ErrField(err))
	}

	// Also call local message handler for processing
	if e.messageHandler != nil {
		if err := e.messageHandler(msg); err != nil {
			return fmt.Errorf("failed to handle new-view locally: %w", err)
		}
	}

	return nil
}

// handleNewView handles New-View messages (Task 5.3.5)
func (e *PBFTEngine) handleNewView(msg *PBFTMessage) error {
	var payload NewViewPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal new-view payload: %w", err)
	}

	// Verify new primary
	e.validators.UpdateView(payload.NewViewNumber)
	newPrimary := e.validators.GetPrimary()
	if msg.Sender != newPrimary {
		return errors.New("new-view not from expected primary")
	}

	// Verify view change proofs have quorum
	quorum := e.validators.Quorum()
	if len(payload.ViewChangeProofs) < quorum {
		return errors.New("insufficient view change proofs")
	}

	// Update view
	e.viewMu.Lock()
	e.view.Number = payload.NewViewNumber
	e.view.Primary = newPrimary
	e.view.Timestamp = time.Now().Unix()
	e.viewMu.Unlock()

	// Update role
	if newPrimary == e.address {
		e.role = RolePrimary
	} else if e.validators.IsValidator(e.address) {
		e.role = RoleReplica
	}

	// Clear view change state
	e.vcMu.Lock()
	e.viewChangeState = nil
	e.vcMu.Unlock()

	// Reset view change timer
	e.startViewChangeTimer()

	logger.Info("View changed",
		logger.Uint64("new_view", payload.NewViewNumber),
		logger.String("new_primary", newPrimary.String()),
		logger.String("role", e.role.String()),
	)

	return nil
}

// startViewChangeTimer starts the view change timer (Task 5.3.1)
func (e *PBFTEngine) startViewChangeTimer() {
	if e.viewChangeTimer != nil {
		e.viewChangeTimer.Stop()
	}

	e.viewChangeTimer = time.AfterFunc(e.config.ViewTimeout, func() {
		e.initiateViewChange()
	})
}

// initiateViewChange initiates a view change (Task 5.3.2)
func (e *PBFTEngine) initiateViewChange() {
	e.viewMu.RLock()
	currentView := e.view.Number
	e.viewMu.RUnlock()

	newView := currentView + 1

	// Create view change payload
	payload := ViewChangePayload{
		NewViewNumber:        newView,
		LastStableCheckpoint: e.lastCheckpoint,
		PreparedProofs:       make([]Proof, 0),
	}

	// Include prepared proofs if any
	e.statesMu.RLock()
	for _, state := range e.states {
		if state.State >= StatePrepared {
			proof := Proof{
				View:      state.View,
				SeqNumber: state.SeqNumber,
				BlockHash: state.BlockHash,
			}
			payload.PreparedProofs = append(payload.PreparedProofs, proof)
		}
	}
	e.statesMu.RUnlock()

	payloadBytes, _ := json.Marshal(payload)

	msg := &PBFTMessage{
		Type:      MsgViewChange,
		View:      currentView,
		Sender:    e.address,
		Payload:   payloadBytes,
		Timestamp: time.Now().Unix(),
	}

	if err := msg.Sign(e.privateKey); err != nil {
		logger.Error("Failed to sign view change", logger.ErrField(err))
		return
	}

	// Broadcast to network
	if err := e.broadcastToNetwork(msg); err != nil {
		logger.Debug("Failed to broadcast view change to network", logger.ErrField(err))
	}

	// Also call local message handler for processing
	if e.messageHandler != nil {
		if err := e.messageHandler(msg); err != nil {
			logger.Error("Failed to handle view change locally", logger.ErrField(err))
		}
	}

	e.viewChangeState = NewViewChangeState(newView)
	e.viewChangeState.AddViewChange(msg)

	logger.Info("Initiated view change",
		logger.Uint64("from_view", currentView),
		logger.Uint64("to_view", newView),
	)
}

// startBlockTimer starts the block production timer for primary
func (e *PBFTEngine) startBlockTimer() {
	ticker := time.NewTicker(e.config.BlockInterval)
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer ticker.Stop()

		for {
			select {
			case <-e.ctx.Done():
				return
			case <-ticker.C:
				if e.role == RolePrimary {
					if err := e.ProposeBlock(); err != nil {
						logger.Debug("Failed to propose block", logger.ErrField(err))
					}
				}
			}
		}
	}()
}

// GetView returns the current view
func (e *PBFTEngine) GetView() *View {
	e.viewMu.RLock()
	defer e.viewMu.RUnlock()
	
	// Return a copy
	return &View{
		Number:    e.view.Number,
		Primary:   e.view.Primary,
		Timestamp: e.view.Timestamp,
	}
}

// GetRole returns the current role
func (e *PBFTEngine) GetRole() NodeRole {
	return e.role
}

// GetValidatorSet returns the current validator set
func (e *PBFTEngine) GetValidatorSet() *ValidatorSet {
	e.valSetMu.RLock()
	defer e.valSetMu.RUnlock()
	
	// Return a copy
	return &ValidatorSet{
		Validators: make([]blockchain.Address, len(e.validators.Validators)),
		ViewNumber: e.validators.ViewNumber,
	}
}

// AddValidator adds a validator (Task 5.4.2)
func (e *PBFTEngine) AddValidator(addr blockchain.Address) error {
	e.valSetMu.Lock()
	defer e.valSetMu.Unlock()

	return e.validators.AddValidator(addr)
}

// RemoveValidator removes a validator (Task 5.4.3)
func (e *PBFTEngine) RemoveValidator(addr blockchain.Address) error {
	e.valSetMu.Lock()
	defer e.valSetMu.Unlock()

	return e.validators.RemoveValidator(addr)
}

// IsValidator checks if address is a validator
func (e *PBFTEngine) IsValidator(addr blockchain.Address) bool {
	e.valSetMu.RLock()
	defer e.valSetMu.RUnlock()

	return e.validators.IsValidator(addr)
}

// GetStats returns consensus statistics
func (e *PBFTEngine) GetStats() map[string]interface{} {
	e.metrics.mu.RLock()
	defer e.metrics.mu.RUnlock()

	return map[string]interface{}{
		"view_changes":     e.metrics.ViewChanges,
		"blocks_committed": e.metrics.BlocksCommitted,
		"current_view":     e.view.Number,
		"current_role":     e.role.String(),
		"validators":       len(e.validators.Validators),
		"quorum":           e.validators.Quorum(),
	}
}
