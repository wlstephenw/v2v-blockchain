package consensus

import (
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// CheckpointManager manages consensus checkpoints (Task 5.5.5)
type CheckpointManager struct {
	engine        *PBFTEngine
	checkpoints   map[uint64]*Checkpoint
	lastStable    uint64
	checkpointCh  chan uint64
	stopCh        chan struct{}
}

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(engine *PBFTEngine) *CheckpointManager {
	return &CheckpointManager{
		engine:       engine,
		checkpoints:  make(map[uint64]*Checkpoint),
		lastStable:   0,
		checkpointCh: make(chan uint64, 10),
		stopCh:       make(chan struct{}),
	}
}

// Start starts the checkpoint manager
func (cm *CheckpointManager) Start() {
	go cm.checkpointLoop()
}

// Stop stops the checkpoint manager
func (cm *CheckpointManager) Stop() {
	close(cm.stopCh)
}

// checkpointLoop processes checkpoint requests
func (cm *CheckpointManager) checkpointLoop() {
	for {
		select {
		case <-cm.stopCh:
			return
		case seqNum := <-cm.checkpointCh:
			cm.createCheckpoint(seqNum)
		}
	}
}

// createCheckpoint creates a checkpoint at the given sequence number
func (cm *CheckpointManager) createCheckpoint(seqNum uint64) {
	// Get the block at this sequence number
	block, err := cm.engine.bc.GetBlockByHeight(seqNum)
	if err != nil {
		logger.Warn("Failed to get block for checkpoint",
			logger.Uint64("seq", seqNum),
			logger.ErrField(err),
		)
		return
	}

	checkpoint := &Checkpoint{
		SeqNumber: seqNum,
		BlockHash: block.Header.Hash,
		Timestamp: time.Now().Unix(),
	}

	cm.checkpoints[seqNum] = checkpoint

	// Clean old checkpoints
	cm.garbageCollectOldCheckpoints(seqNum)

	logger.Info("Checkpoint created",
		logger.Uint64("seq", seqNum),
		logger.String("hash", block.Header.Hash.String()),
	)
}

// garbageCollectOldCheckpoints removes old checkpoints
func (cm *CheckpointManager) garbageCollectOldCheckpoints(currentStable uint64) {
	for seqNum := range cm.checkpoints {
		if seqNum < currentStable {
			delete(cm.checkpoints, seqNum)
		}
	}
	cm.lastStable = currentStable
}

// GetCheckpoint returns the checkpoint at the given sequence number
func (cm *CheckpointManager) GetCheckpoint(seqNum uint64) (*Checkpoint, bool) {
	cp, exists := cm.checkpoints[seqNum]
	return cp, exists
}

// GetLastStableCheckpoint returns the last stable checkpoint
func (cm *CheckpointManager) GetLastStableCheckpoint() *Checkpoint {
	return cm.checkpoints[cm.lastStable]
}

// RequestCheckpoint requests a checkpoint to be created
func (cm *CheckpointManager) RequestCheckpoint(seqNum uint64) {
	select {
	case cm.checkpointCh <- seqNum:
	default:
		logger.Warn("Checkpoint channel full, dropping request")
	}
}
