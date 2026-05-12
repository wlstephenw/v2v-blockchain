package network

import (
	"sync"
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

const (
	// PartitionCheckInterval is how often to check for network partitions
	PartitionCheckInterval = 10 * time.Second

	// MinPeersForConnected is the minimum peers to consider connected to network
	MinPeersForConnected = 2

	// PartitionRecoveryTimeout is how long to wait before attempting recovery
	PartitionRecoveryTimeout = 30 * time.Second

	// MaxPartitionDuration is the maximum time to tolerate being partitioned
	MaxPartitionDuration = 5 * time.Minute
)

// PartitionState represents the network partition state
type PartitionState int

const (
	// StateConnected means we're connected to the network
	StateConnected PartitionState = iota

	// StatePartitioned means we've detected a network partition
	StatePartitioned

	// StateRecovering means we're attempting to recover from a partition
	StateRecovering
)

// String returns the string representation of the partition state
func (s PartitionState) String() string {
	switch s {
	case StateConnected:
		return "connected"
	case StatePartitioned:
		return "partitioned"
	case StateRecovering:
		return "recovering"
	default:
		return "unknown"
	}
}

// PartitionDetector monitors network connectivity and detects partitions
type PartitionDetector struct {
	host           *Host
	logger         *logger.Logger
	state          PartitionState
	stateMu        sync.RWMutex
	partitionStart time.Time
	lastPeerCount  int
	checkInterval  time.Duration
	stopCh         chan struct{}
	wg             sync.WaitGroup

	// Callbacks
	onPartitionDetected func()
	onRecoveryComplete  func()
}

// NewPartitionDetector creates a new partition detector
func NewPartitionDetector(h *Host, log *logger.Logger) *PartitionDetector {
	return &PartitionDetector{
		host:          h,
		logger:        log,
		state:         StateConnected,
		checkInterval: PartitionCheckInterval,
		stopCh:        make(chan struct{}),
	}
}

// Start starts the partition detector
func (pd *PartitionDetector) Start() error {
	pd.logger.Info("Starting partition detector")

	pd.wg.Add(1)
	go pd.detectionLoop()

	return nil
}

// Stop stops the partition detector
func (pd *PartitionDetector) Stop() error {
	pd.logger.Info("Stopping partition detector")
	close(pd.stopCh)
	pd.wg.Wait()
	return nil
}

// GetState returns the current partition state
func (pd *PartitionDetector) GetState() PartitionState {
	pd.stateMu.RLock()
	defer pd.stateMu.RUnlock()
	return pd.state
}

// IsPartitioned returns true if the network is partitioned
func (pd *PartitionDetector) IsPartitioned() bool {
	return pd.GetState() == StatePartitioned
}

// SetPartitionCallback sets the callback for partition detection
func (pd *PartitionDetector) SetPartitionCallback(cb func()) {
	pd.onPartitionDetected = cb
}

// SetRecoveryCallback sets the callback for recovery completion
func (pd *PartitionDetector) SetRecoveryCallback(cb func()) {
	pd.onRecoveryComplete = cb
}

// detectionLoop periodically checks for network partitions
func (pd *PartitionDetector) detectionLoop() {
	defer pd.wg.Done()

	ticker := time.NewTicker(pd.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pd.stopCh:
			return
		case <-ticker.C:
			pd.checkPartition()
		}
	}
}

// checkPartition checks if we're partitioned from the network
func (pd *PartitionDetector) checkPartition() {
	peerCount := pd.host.PeerCount()
	currentState := pd.GetState()

	switch currentState {
	case StateConnected:
		if peerCount < MinPeersForConnected {
			// Potential partition detected
			if pd.lastPeerCount < MinPeersForConnected {
				// Confirmed partition - we had low peers in the last check too
				pd.setPartitioned()
			}
		}

	case StatePartitioned:
		if peerCount >= MinPeersForConnected {
			// Recovery detected
			pd.setRecovered()
		} else {
			// Check if we've been partitioned too long
			if time.Since(pd.partitionStart) > MaxPartitionDuration {
				pd.logger.Warn("Network partition exceeded maximum duration",
					logger.Duration("duration", time.Since(pd.partitionStart)),
				)
				// Force recovery attempt
				pd.attemptRecovery()
			}
		}

	case StateRecovering:
		if peerCount >= MinPeersForConnected {
			// Recovery successful
			pd.setRecovered()
		} else if time.Since(pd.partitionStart) > PartitionRecoveryTimeout {
			// Recovery failed, back to partitioned state
			pd.setPartitioned()
		}
	}

	pd.lastPeerCount = peerCount
}

// setPartitioned sets the state to partitioned
func (pd *PartitionDetector) setPartitioned() {
	pd.stateMu.Lock()
	pd.state = StatePartitioned
	pd.partitionStart = time.Now()
	pd.stateMu.Unlock()

	pd.logger.Warn("Network partition detected",
		logger.Int("peer_count", pd.host.PeerCount()),
		logger.Int("min_required", MinPeersForConnected),
	)

	// Trigger callback if set
	if pd.onPartitionDetected != nil {
		go pd.onPartitionDetected()
	}

	// Attempt immediate recovery
	pd.attemptRecovery()
}

// setRecovered sets the state to connected
func (pd *PartitionDetector) setRecovered() {
	pd.stateMu.Lock()
	prevState := pd.state
	pd.state = StateConnected
	partitionDuration := time.Since(pd.partitionStart)
	pd.stateMu.Unlock()

	if prevState != StateConnected {
		pd.logger.Info("Network partition recovered",
			logger.Int("peer_count", pd.host.PeerCount()),
			logger.Duration("partition_duration", partitionDuration),
		)

		// Trigger callback if set
		if pd.onRecoveryComplete != nil {
			go pd.onRecoveryComplete()
		}
	}
}

// attemptRecovery attempts to recover from a network partition
func (pd *PartitionDetector) attemptRecovery() {
	pd.stateMu.Lock()
	if pd.state == StateRecovering {
		pd.stateMu.Unlock()
		return
	}
	pd.state = StateRecovering
	pd.stateMu.Unlock()

	pd.logger.Info("Attempting network partition recovery")

	// 1. Try to reconnect to bootstrap peers
	if err := pd.host.connectToBootstrapPeers(); err != nil {
		pd.logger.Debug("Bootstrap reconnection failed", logger.ErrField(err))
	}

	// 2. Trigger DHT discovery if available
	if pd.host.discovery != nil {
		pd.host.discovery.FindPeersViaDHT()
	}

	// 3. Refresh peer connections
	pd.refreshPeerConnections()
}

// refreshPeerConnections attempts to refresh connections to known peers
func (pd *PartitionDetector) refreshPeerConnections() {
	peers := pd.host.GetPeers()
	pd.logger.Debug("Refreshing peer connections", logger.Int("known_peers", len(peers)))

	for _, peerInfo := range peers {
		// Skip if already connected
		if pd.host.host.Network().Connectedness(peerInfo.ID) == pd.host.host.Network().Connectedness(peerInfo.ID) {
			continue
		}

		// Try to reconnect
		if len(peerInfo.Multiaddrs) > 0 {
			ctx, cancel := pd.host.ctx, func() {}
			if len(peerInfo.Multiaddrs) > 0 {
				addr := peerInfo.Multiaddrs[0]
				if err := pd.host.Connect(ctx, addr); err != nil {
					pd.logger.Debug("Failed to reconnect to peer",
						logger.String("peer", peerInfo.ID.String()),
						logger.ErrField(err),
					)
				}
			}
			cancel()
		}
	}
}

// GetPartitionStats returns statistics about partition detection
func (pd *PartitionDetector) GetPartitionStats() PartitionStats {
	pd.stateMu.RLock()
	defer pd.stateMu.RUnlock()

	return PartitionStats{
		State:         pd.state.String(),
		PeerCount:     pd.host.PeerCount(),
		PartitionTime: pd.partitionStart,
		IsPartitioned: pd.state == StatePartitioned,
	}
}

// PartitionStats contains partition detection statistics
type PartitionStats struct {
	State         string    `json:"state"`
	PeerCount     int       `json:"peer_count"`
	PartitionTime time.Time `json:"partition_time"`
	IsPartitioned bool      `json:"is_partitioned"`
}
