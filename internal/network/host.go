package network

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"

	"github.com/v2v-blockchain/v2v-blockchain/internal/config"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

const (
	// ProtocolID is the protocol identifier for V2V blockchain
	ProtocolID = protocol.ID("/v2v-blockchain/1.0.0")

	// DiscoveryServiceTag is the mDNS discovery service tag
	DiscoveryServiceTag = "v2v-blockchain"
)

// Host wraps a libp2p host with additional V2V blockchain functionality
type Host struct {
	host   host.Host
	config *config.Config
	logger *logger.Logger

	// Connection management
	peers     map[peer.ID]*PeerInfo
	peersMu   sync.RWMutex
	minConn   int
	maxConn   int

	// Discovery
	discovery *DiscoveryService

	// Gossip
	gossip *GossipService

	// Partition Detection
	partitionDetector *PartitionDetector

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// PeerInfo holds information about a connected peer
type PeerInfo struct {
	ID          peer.ID
	Multiaddrs  []multiaddr.Multiaddr
	ConnectedAt time.Time
	Direction   network.Direction
	Protocols   []string
}

// NewHost creates a new libp2p host with V2V blockchain configuration
func NewHost(cfg *config.Config, log *logger.Logger) (*Host, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create or load host key
	privKey, err := generateOrLoadKey(cfg)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to generate/load host key: %w", err)
	}

	// Parse listen addresses
	listenAddrs, err := parseListenAddrs(cfg.Node.ListenAddr)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to parse listen addresses: %w", err)
	}

	// Create libp2p host options
	opts := []libp2p.Option{
		libp2p.Identity(privKey),
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
		libp2p.Ping(true),
		libp2p.ProtocolVersion("1.0.0"),
	}

	// Create the libp2p host
	libp2pHost, err := libp2p.New(opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	h := &Host{
		host:    libp2pHost,
		config:  cfg,
		logger:  log,
		peers:   make(map[peer.ID]*PeerInfo),
		minConn: cfg.Network.MinConnections,
		maxConn: cfg.Network.MaxConnections,
		ctx:     ctx,
		cancel:  cancel,
	}

	// Set up connection event handlers
	h.setupConnectionHandlers()

	log.Info("Libp2p host created successfully",
		logger.String("id", libp2pHost.ID().String()),
		logger.Int("min_connections", h.minConn),
		logger.Int("max_connections", h.maxConn),
	)

	return h, nil
}

// Start starts the host and begins background tasks
func (h *Host) Start() error {
	h.logger.Info("Starting P2P network host")

	// Initialize and start discovery service
	discoverySvc, err := NewDiscoveryService(h, h.logger)
	if err != nil {
		return fmt.Errorf("failed to create discovery service: %w", err)
	}
	h.discovery = discoverySvc

	if err := h.discovery.Start(); err != nil {
		return fmt.Errorf("failed to start discovery service: %w", err)
	}

	// Advertise this node in DHT
	if h.config.Network.DHTEnabled {
		if err := h.discovery.Advertise(); err != nil {
			h.logger.Warn("Failed to advertise in DHT", logger.ErrField(err))
		}
	}

	// Initialize and start gossip service
	gossipSvc, err := NewGossipService(h, h.logger)
	if err != nil {
		return fmt.Errorf("failed to create gossip service: %w", err)
	}
	h.gossip = gossipSvc

	if err := h.gossip.Start(); err != nil {
		return fmt.Errorf("failed to start gossip service: %w", err)
	}

	// Initialize and start partition detector
	partitionDetector := NewPartitionDetector(h, h.logger)
	h.partitionDetector = partitionDetector

	if err := h.partitionDetector.Start(); err != nil {
		return fmt.Errorf("failed to start partition detector: %w", err)
	}

	// Start connection management loop
	h.wg.Add(1)
	go h.connectionManager()

	// Connect to bootstrap peers
	if len(h.config.Network.BootstrapPeers) > 0 {
		if err := h.connectToBootstrapPeers(); err != nil {
			h.logger.Warn("Failed to connect to some bootstrap peers", logger.ErrField(err))
		}
	}

	h.logger.Info("P2P network host started",
		logger.String("id", h.host.ID().String()),
		logger.Int("connected_peers", len(h.host.Network().Peers())),
	)

	return nil
}

// Stop gracefully stops the host
func (h *Host) Stop() error {
	h.logger.Info("Stopping P2P network host")

	// Signal shutdown
	h.cancel()

	// Stop gossip service
	if h.gossip != nil {
		if err := h.gossip.Stop(); err != nil {
			h.logger.Warn("Error stopping gossip service", logger.ErrField(err))
		}
	}

	// Stop partition detector
	if h.partitionDetector != nil {
		if err := h.partitionDetector.Stop(); err != nil {
			h.logger.Warn("Error stopping partition detector", logger.ErrField(err))
		}
	}

	// Stop discovery service
	if h.discovery != nil {
		if err := h.discovery.Stop(); err != nil {
			h.logger.Warn("Error stopping discovery service", logger.ErrField(err))
		}
	}

	// Wait for background tasks to complete
	done := make(chan struct{})
	go func() {
		h.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		h.logger.Info("All background tasks stopped")
	case <-time.After(10 * time.Second):
		h.logger.Warn("Timeout waiting for background tasks")
	}

	// Close the host
	if err := h.host.Close(); err != nil {
		return fmt.Errorf("failed to close host: %w", err)
	}

	h.logger.Info("P2P network host stopped")
	return nil
}

// ID returns the host's peer ID
func (h *Host) ID() peer.ID {
	return h.host.ID()
}

// Addrs returns the host's listen addresses
func (h *Host) Addrs() []multiaddr.Multiaddr {
	return h.host.Addrs()
}

// Host returns the underlying libp2p host
func (h *Host) Host() host.Host {
	return h.host
}

// Gossip returns the gossip service
func (h *Host) Gossip() *GossipService {
	return h.gossip
}

// Connect connects to a peer at the given multiaddress
func (h *Host) Connect(ctx context.Context, addr multiaddr.Multiaddr) error {
	peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("failed to parse peer address: %w", err)
	}

	if err := h.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer %s: %w", peerInfo.ID, err)
	}

	h.logger.Debug("Connected to peer", logger.String("peer", peerInfo.ID.String()))
	return nil
}

// GetPeers returns information about connected peers
func (h *Host) GetPeers() []*PeerInfo {
	h.peersMu.RLock()
	defer h.peersMu.RUnlock()

	peers := make([]*PeerInfo, 0, len(h.peers))
	for _, info := range h.peers {
		peers = append(peers, info)
	}
	return peers
}

// PeerCount returns the number of connected peers
func (h *Host) PeerCount() int {
	return len(h.host.Network().Peers())
}

// GetNetworkStats returns network statistics
func (h *Host) GetNetworkStats() NetworkStats {
	stats := NetworkStats{
		PeerCount:       h.PeerCount(),
		ConnectedPeers:  h.GetPeers(),
		MinConnections:  h.minConn,
		MaxConnections:  h.maxConn,
	}

	if h.partitionDetector != nil {
		stats.PartitionStats = h.partitionDetector.GetPartitionStats()
	}

	return stats
}

// NetworkStats holds network statistics
type NetworkStats struct {
	PeerCount      int             `json:"peer_count"`
	ConnectedPeers []*PeerInfo     `json:"connected_peers"`
	MinConnections int             `json:"min_connections"`
	MaxConnections int             `json:"max_connections"`
	PartitionStats PartitionStats  `json:"partition_stats,omitempty"`
}

// setupConnectionHandlers sets up connection event handlers
func (h *Host) setupConnectionHandlers() {
	h.host.Network().Notify(&network.NotifyBundle{
		ConnectedF: func(n network.Network, conn network.Conn) {
			peerID := conn.RemotePeer()
			h.peersMu.Lock()
			h.peers[peerID] = &PeerInfo{
				ID:          peerID,
				Multiaddrs:  n.Peerstore().Addrs(peerID),
				ConnectedAt: time.Now(),
				Direction:   conn.Stat().Direction,
			}
			h.peersMu.Unlock()

			h.logger.Debug("Peer connected",
				logger.String("peer", peerID.String()),
				logger.String("direction", conn.Stat().Direction.String()),
			)
		},
		DisconnectedF: func(n network.Network, conn network.Conn) {
			peerID := conn.RemotePeer()
			h.peersMu.Lock()
			delete(h.peers, peerID)
			h.peersMu.Unlock()

			h.logger.Debug("Peer disconnected",
				logger.String("peer", peerID.String()),
			)
		},
	})
}

// connectionManager manages peer connections
func (h *Host) connectionManager() {
	defer h.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.maintainConnections()
		}
	}
}

// maintainConnections ensures connection count stays within limits and triggers discovery
func (h *Host) maintainConnections() {
	peerCount := h.PeerCount()

	if peerCount < h.minConn {
		h.logger.Debug("Low peer count, attempting to discover more peers",
			logger.Int("current", peerCount),
			logger.Int("minimum", h.minConn),
		)
		// Trigger discovery if available
		if h.discovery != nil && h.discovery.DHT != nil {
			h.discovery.FindPeersViaDHT()
		}
	} else if peerCount > h.maxConn {
		h.logger.Debug("High peer count, trimming connections",
			logger.Int("current", peerCount),
			logger.Int("maximum", h.maxConn),
		)
		// Trim excess connections
		peers := h.host.Network().Peers()
		for i := 0; i < len(peers)-h.maxConn && i < len(peers); i++ {
			if err := h.host.Network().ClosePeer(peers[i]); err != nil {
				h.logger.Warn("Failed to close peer connection",
					logger.String("peer", peers[i].String()),
					logger.ErrField(err),
				)
			}
		}
	}
}

// connectToBootstrapPeers connects to configured bootstrap peers
func (h *Host) connectToBootstrapPeers() error {
	for _, addr := range h.config.Network.BootstrapPeers {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			h.logger.Warn("Invalid bootstrap peer address",
				logger.String("addr", addr),
				logger.ErrField(err),
			)
			continue
		}

		ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
		if err := h.Connect(ctx, maddr); err != nil {
			h.logger.Warn("Failed to connect to bootstrap peer",
				logger.String("addr", addr),
				logger.ErrField(err),
			)
		}
		cancel()
	}
	return nil
}

// generateOrLoadKey generates or loads the host's private key
func generateOrLoadKey(cfg *config.Config) (crypto.PrivKey, error) {
	// For now, generate a new key each time
	// In production, this should load from file or secure storage
	privKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
	if err != nil {
		return nil, err
	}
	return privKey, nil
}

// parseListenAddrs parses listen address strings
func parseListenAddrs(addrs ...string) ([]multiaddr.Multiaddr, error) {
	maddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
	for _, addr := range addrs {
		maddr, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid address %s: %w", addr, err)
		}
		maddrs = append(maddrs, maddr)
	}
	return maddrs, nil
}
