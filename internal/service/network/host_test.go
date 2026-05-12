package network

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/v2v-blockchain/v2v-blockchain/internal/app/config"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

func TestNewHost(t *testing.T) {
	log, err := logger.New(config.LogConfig{
		Level:  "debug",
		Format: "console",
		Output: "stdout",
	})
	require.NoError(t, err)
	defer log.Sync()
	logger.SetDefault(log)

	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:         "test-node",
			Role:       "validator",
			ListenAddr: "/ip4/127.0.0.1/tcp/0",
		},
		Network: config.NetworkConfig{
			MinConnections: 8,
			MaxConnections: 12,
			MDNSEnabled:    false,
		},
	}

	host, err := NewHost(cfg, log)
	require.NoError(t, err)
	defer host.Stop()

	// Verify host was created
	assert.NotNil(t, host.host)
	assert.NotEmpty(t, host.ID())
	assert.NotEmpty(t, host.Addrs())

	// Verify configuration
	assert.Equal(t, 8, host.minConn)
	assert.Equal(t, 12, host.maxConn)
}

func TestHostStartStop(t *testing.T) {
	log, err := logger.New(config.LogConfig{
		Level:  "error",
		Format: "console",
		Output: "stdout",
	})
	require.NoError(t, err)
	defer log.Sync()

	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:         "test-node",
			Role:       "validator",
			ListenAddr: "/ip4/127.0.0.1/tcp/0",
		},
		Network: config.NetworkConfig{
			MinConnections: 8,
			MaxConnections: 12,
			MDNSEnabled:    false,
		},
	}

	host, err := NewHost(cfg, log)
	require.NoError(t, err)

	// Test start
	err = host.Start()
	require.NoError(t, err)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Test stop
	err = host.Stop()
	require.NoError(t, err)
}

func TestHostPeerConnection(t *testing.T) {
	log, err := logger.New(config.LogConfig{
		Level:  "error",
		Format: "console",
		Output: "stdout",
	})
	require.NoError(t, err)
	defer log.Sync()

	// Create two hosts
	cfg1 := &config.Config{
		Node: config.NodeConfig{
			ID:         "node-1",
			Role:       "validator",
			ListenAddr: "/ip4/127.0.0.1/tcp/0",
		},
		Network: config.NetworkConfig{
			MinConnections: 8,
			MaxConnections: 12,
			MDNSEnabled:    false,
		},
	}

	cfg2 := &config.Config{
		Node: config.NodeConfig{
			ID:         "node-2",
			Role:       "validator",
			ListenAddr: "/ip4/127.0.0.1/tcp/0",
		},
		Network: config.NetworkConfig{
			MinConnections: 8,
			MaxConnections: 12,
			MDNSEnabled:    false,
		},
	}

	host1, err := NewHost(cfg1, log)
	require.NoError(t, err)
	defer host1.Stop()

	host2, err := NewHost(cfg2, log)
	require.NoError(t, err)
	defer host2.Stop()

	err = host1.Start()
	require.NoError(t, err)

	err = host2.Start()
	require.NoError(t, err)

	// Get host1's addresses
	addrs1 := host1.Addrs()
	require.NotEmpty(t, addrs1)

	// Create full address with peer ID
	peerIDAddr, err := multiaddr.NewMultiaddr("/p2p/" + host1.ID().String())
	require.NoError(t, err)
	peerInfo1 := addrs1[0].Encapsulate(peerIDAddr)

	// Connect host2 to host1
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = host2.Connect(ctx, peerInfo1)
	require.NoError(t, err)

	// Give it a moment to establish connection
	time.Sleep(200 * time.Millisecond)

	// Verify connection
	assert.True(t, host2.host.Network().Connectedness(host1.ID()) == network.Connected)
}

func TestParseListenAddrs(t *testing.T) {
	tests := []struct {
		name    string
		addrs   []string
		wantErr bool
	}{
		{
			name:    "valid tcp address",
			addrs:   []string{"/ip4/0.0.0.0/tcp/10001"},
			wantErr: false,
		},
		{
			name:    "valid multiple addresses",
			addrs:   []string{"/ip4/0.0.0.0/tcp/10001", "/ip6/::/tcp/10001"},
			wantErr: false,
		},
		{
			name:    "invalid address",
			addrs:   []string{"invalid-address"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			maddrs, err := parseListenAddrs(tt.addrs...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, maddrs, len(tt.addrs))
			}
		})
	}
}
