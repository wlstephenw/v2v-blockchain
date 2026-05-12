package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

const (
	// DiscoveryNamespace is the DHT namespace for V2V blockchain
	DiscoveryNamespace = "/v2v-blockchain/discovery"

	// RendezvousString is used for DHT discovery
	RendezvousString = "v2v-blockchain-rendezvous"

	// DiscoveryInterval is how often to search for peers
	DiscoveryInterval = 30 * time.Second
)

// DiscoveryService manages peer discovery using DHT and mDNS
type DiscoveryService struct {
	host      *Host
	DHT       *dht.IpfsDHT
	mdns      mdns.Service
	logger    *logger.Logger
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	isRunning bool
	mu        sync.RWMutex
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(h *Host, log *logger.Logger) (*DiscoveryService, error) {
	ctx, cancel := context.WithCancel(h.ctx)

	return &DiscoveryService{
		host:   h,
		logger: log,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start starts the discovery service
func (d *DiscoveryService) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isRunning {
		return nil
	}

	d.logger.Info("Starting discovery service")

	// Initialize DHT if enabled
	if d.host.config.Network.DHTEnabled {
		if err := d.setupDHT(); err != nil {
			return fmt.Errorf("failed to setup DHT: %w", err)
		}
	}

	// Initialize mDNS if enabled
	if d.host.config.Network.MDNSEnabled {
		if err := d.setupMDNS(); err != nil {
			return fmt.Errorf("failed to setup mDNS: %w", err)
		}
	}

	// Start discovery loop
	d.wg.Add(1)
	go d.discoveryLoop()

	d.isRunning = true
	d.logger.Info("Discovery service started",
		logger.Bool("dht_enabled", d.host.config.Network.DHTEnabled),
		logger.Bool("mdns_enabled", d.host.config.Network.MDNSEnabled),
	)

	return nil
}

// Stop stops the discovery service
func (d *DiscoveryService) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.isRunning {
		return nil
	}

	d.logger.Info("Stopping discovery service")

	// Cancel context to stop goroutines
	d.cancel()

	// Wait for discovery loop to finish
	d.wg.Wait()

	// Stop DHT
	if d.DHT != nil {
		if err := d.DHT.Close(); err != nil {
			d.logger.Warn("Error closing DHT", logger.ErrField(err))
		}
	}

	// Stop mDNS
	if d.mdns != nil {
		if err := d.mdns.Close(); err != nil {
			d.logger.Warn("Error closing mDNS", logger.ErrField(err))
		}
	}

	d.isRunning = false
	d.logger.Info("Discovery service stopped")
	return nil
}

// setupDHT initializes the DHT for peer discovery
func (d *DiscoveryService) setupDHT() error {
	// Determine if this is a bootstrap node
	isBootstrap := len(d.host.config.Network.BootstrapPeers) == 0

	var dhtOpts []dht.Option
	if isBootstrap {
		// Bootstrap node - start in server mode
		dhtOpts = append(dhtOpts, dht.Mode(dht.ModeServer))
		d.logger.Info("Running as DHT bootstrap node")
	} else {
		// Regular node - start in client mode
		dhtOpts = append(dhtOpts, dht.Mode(dht.ModeClient))
	}

	// Create DHT instance
	newDHT, err := dht.New(d.ctx, d.host.host, dhtOpts...)
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}

	d.DHT = newDHT

	// Bootstrap the DHT
	if err := d.DHT.Bootstrap(d.ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	d.logger.Info("DHT initialized")
	return nil
}

// setupMDNS initializes mDNS for local peer discovery
func (d *DiscoveryService) setupMDNS() error {
	service := mdns.NewMdnsService(d.host.host, DiscoveryServiceTag, &mdnsNotifee{host: d.host})
	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start mDNS service: %w", err)
	}

	d.mdns = service
	d.logger.Info("mDNS discovery initialized")
	return nil
}

// discoveryLoop periodically searches for new peers
func (d *DiscoveryService) discoveryLoop() {
	defer d.wg.Done()

	ticker := time.NewTicker(DiscoveryInterval)
	defer ticker.Stop()

	// Initial discovery
	d.findPeers()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.findPeers()
		}
	}
}

// findPeers searches for and connects to new peers
func (d *DiscoveryService) findPeers() {
	// Check if we need more connections
	currentPeers := d.host.PeerCount()
	if currentPeers >= d.host.minConn {
		d.logger.Debug("Sufficient peer connections", logger.Int("count", currentPeers))
		return
	}

	d.logger.Debug("Searching for peers", logger.Int("current_peers", currentPeers))

	// DHT peer discovery is handled by the routing table updates
	// Additional discovery can be triggered here if needed
}

// FindPeersViaDHT searches for peers using the DHT routing table
func (d *DiscoveryService) FindPeersViaDHT() {
	if d.DHT == nil {
		return
	}

	// Get peers from routing table
	peers := d.DHT.RoutingTable().ListPeers()
	d.logger.Debug("Found peers in DHT routing table", logger.Int("count", len(peers)))

	for _, peerID := range peers {
		// Skip self
		if peerID == d.host.ID() {
			continue
		}

		// Skip already connected peers
		if d.host.host.Network().Connectedness(peerID) == network.Connected {
			continue
		}

		// Get peer addresses from peerstore
		addrs := d.host.host.Peerstore().Addrs(peerID)
		if len(addrs) == 0 {
			continue
		}

		peerInfo := peer.AddrInfo{
			ID:    peerID,
			Addrs: addrs,
		}

		// Try to connect
		connectCtx, connectCancel := context.WithTimeout(d.ctx, 5*time.Second)
		if err := d.host.host.Connect(connectCtx, peerInfo); err != nil {
			d.logger.Debug("Failed to connect to discovered peer",
				logger.String("peer", peerID.String()),
				logger.ErrField(err),
			)
		} else {
			d.logger.Debug("Connected to peer via DHT", logger.String("peer", peerID.String()))
		}
		connectCancel()
	}
}

// Advertise advertises this node in the DHT by ensuring we're in the routing table
func (d *DiscoveryService) Advertise() error {
	if d.DHT == nil {
		return fmt.Errorf("DHT not initialized")
	}

	// The DHT automatically advertises the node when it bootsraps
	// Additional advertising can be done by refreshing the routing table
	d.logger.Info("DHT advertising active",
		logger.String("rendezvous", RendezvousString),
		logger.Int("routing_table_size", d.DHT.RoutingTable().Size()),
	)
	return nil
}

// GetRoutingTableSize returns the number of peers in the routing table
func (d *DiscoveryService) GetRoutingTableSize() int {
	if d.DHT == nil {
		return 0
	}
	return d.DHT.RoutingTable().Size()
}

// FindPeers finds peers matching a specific criteria using the routing table
func (d *DiscoveryService) FindPeers(ctx context.Context) ([]peer.ID, error) {
	if d.DHT == nil {
		return nil, fmt.Errorf("DHT not initialized")
	}

	return d.DHT.RoutingTable().ListPeers(), nil
}

// mdnsNotifee handles mDNS discovery events
type mdnsNotifee struct {
	host *Host
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.host.host.ID() {
		return
	}

	n.host.logger.Debug("Discovered peer via mDNS", logger.String("peer", pi.ID.String()))

	// Skip if already connected
	if n.host.host.Network().Connectedness(pi.ID) == network.Connected {
		return
	}

	ctx, cancel := context.WithTimeout(n.host.ctx, 5*time.Second)
	defer cancel()

	if err := n.host.host.Connect(ctx, pi); err != nil {
		n.host.logger.Debug("Failed to connect to discovered peer",
			logger.String("peer", pi.ID.String()),
			logger.ErrField(err),
		)
	}
}
