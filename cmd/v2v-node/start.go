package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/v2v-blockchain/v2v-blockchain/internal/app/api"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/app/config"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/consensus"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/executor"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/identity"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/message"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/network"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/platoon"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/state"
	"github.com/v2v-blockchain/v2v-blockchain/internal/infra/storage"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/txpool"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// newStartCmd creates the start command (Task 10.1)
func newStartCmd() *cobra.Command {
	var (
		apiPort   int
		p2pPort   int
		bootstrap []string
		validator bool
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the V2V blockchain node",
		Long:  `Start the V2V blockchain node with all services (P2P, consensus, API).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStart(apiPort, p2pPort, bootstrap, validator)
		},
	}

	cmd.Flags().IntVar(&apiPort, "api-port", 8080, "API server port")
	cmd.Flags().IntVar(&p2pPort, "p2p-port", 10000, "P2P network port")
	cmd.Flags().StringArrayVar(&bootstrap, "bootstrap", nil, "Bootstrap peer addresses")
	cmd.Flags().BoolVar(&validator, "validator", false, "Run as validator node")

	return cmd
}

func runStart(apiPort, p2pPort int, bootstrap []string, validator bool) error {
	// Initialize logger
	logCfg := config.LogConfig{
		Level:  logLevel,
		Output: "stdout",
	}
	if err := logger.InitDefault(logCfg); err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}

	logger.Info("Starting V2V Blockchain Node",
		logger.String("version", version),
		logger.Int("api_port", apiPort),
		logger.Int("p2p_port", p2pPort),
		logger.Bool("validator", validator),
	)

	// Load configuration
	if configFile != "" {
		logger.Info("Loading config from file", logger.String("file", configFile))
	}

	// Initialize storage
	storeCfg := config.StorageConfig{
		Path: dataDir,
	}
	store, err := storage.NewLevelDBStorage(storeCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	defer store.Close()

	// Initialize blockchain
	bcCfg := &config.BlockchainConfig{}
	bc, err := blockchain.NewBlockchain(bcCfg, store)
	if err != nil {
		return fmt.Errorf("failed to initialize blockchain: %w", err)
	}

	// Initialize identity service
	idService, err := identity.NewService(store, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize identity service: %w", err)
	}

	// Initialize platoon service
	platoonService, err := platoon.NewService(store)
	if err != nil {
		return fmt.Errorf("failed to initialize platoon service: %w", err)
	}

	// Initialize state service
	stateService := state.NewService(store)
	defer stateService.Stop()

	// Initialize message service
	msgService := message.NewService(idService)
	defer msgService.Stop()

	// Initialize transaction pool
	txPool := txpool.NewTxPool(txpool.DefaultTxPoolConfig())
	defer txPool.Stop()

	// Generate node identity (key pair)
	nodeKey, nodeAddr, err := generateNodeIdentity()
	if err != nil {
		return fmt.Errorf("failed to generate node identity: %w", err)
	}

	logger.Info("Node identity generated",
		logger.String("address", nodeAddr.String()),
	)

	// Initialize P2P network host
	cfg := &config.Config{
		Node: config.NodeConfig{
			ID:         nodeAddr.String(),
			Role:       "validator",
			ListenAddr: fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", p2pPort),
		},
		Network: config.NetworkConfig{
			MinConnections: 1,
			MaxConnections: 50,
			DHTEnabled:     true,
			BootstrapPeers: bootstrap,
		},
	}

	p2pHost, err := network.NewHost(cfg, logger.GetDefault())
	if err != nil {
		return fmt.Errorf("failed to create P2P host: %w", err)
	}

	if err := p2pHost.Start(); err != nil {
		return fmt.Errorf("failed to start P2P host: %w", err)
	}
	defer p2pHost.Stop()

	logger.Info("P2P network host started",
		logger.String("peer_id", p2pHost.ID().String()),
		logger.Int("connected_peers", p2pHost.PeerCount()),
	)

	// Initialize consensus (if validator)
	var consensusEngine *consensus.PBFTEngine
	if validator {
		// Create validator set (for now, single node - will be updated when peers join)
		validators := []blockchain.Address{nodeAddr}

		consensusEngine, err = consensus.NewPBFTEngine(
			nodeAddr,
			nodeKey,
			bc,
			store,
			validators,
			consensus.DefaultPBFTConfig(),
		)
		if err != nil {
			return fmt.Errorf("failed to initialize consensus: %w", err)
		}

		// Set up consensus message broadcasting via P2P gossip
		consensusEngine.SetBroadcastFunc(func(data []byte) error {
			return p2pHost.Gossip().PublishConsensusMessage(data)
		})

		// Set up consensus message receiving from P2P gossip
		p2pHost.Gossip().SetConsensusHandler(func(data []byte) error {
			var msg consensus.PBFTMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				return fmt.Errorf("failed to unmarshal consensus message: %w", err)
			}
			return consensusEngine.HandleMessage(&msg)
		})

		// Set up message handler for local processing loop
		consensusEngine.SetMessageHandler(func(msg *consensus.PBFTMessage) error {
			return consensusEngine.HandleMessage(msg)
		})

		// Initialize executor for block execution
		exec := executor.NewExecutor(platoonService, idService, stateService)

		// Set up block commit handler to execute transactions
		consensusEngine.SetBlockCommitHandler(func(block *blockchain.Block) error {
			return exec.ExecuteBlock(block)
		})

		consensusEngine.Start()
		defer consensusEngine.Stop()

		logger.Info("PBFT consensus engine started",
			logger.String("role", consensusEngine.GetRole().String()),
			logger.Int("validators", len(validators)),
		)
	}

	// Initialize API server
	apiConfig := api.DefaultServerConfig()
	apiConfig.Port = apiPort
	apiServer := api.NewServer(
		apiConfig,
		bc,
		store,
		txPool,
		idService,
		platoonService,
		consensusEngine,
		stateService,
		msgService,
	)

	if err := apiServer.Start(); err != nil {
		return fmt.Errorf("failed to start API server: %w", err)
	}
	defer apiServer.Stop()

	logger.Info("V2V Blockchain Node started successfully",
		logger.Int("api_port", apiPort),
		logger.Int("p2p_port", p2pPort),
	)

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("Received shutdown signal", logger.String("signal", sig.String()))
	case <-context.Background().Done():
	}

	logger.Info("V2V Blockchain Node stopped")
	return nil
}

// generateNodeIdentity generates a new ECDSA key pair for the node
func generateNodeIdentity() ([]byte, blockchain.Address, error) {
	// Generate private key
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, blockchain.Address{}, fmt.Errorf("failed to generate key: %w", err)
	}

	// Serialize private key
	privateKeyBytes := privateKey.D.Bytes()

	// Derive address from public key
	publicKeyBytes := elliptic.Marshal(privateKey.Curve, privateKey.X, privateKey.Y)
	
	// Create address (hash of public key)
	var addr blockchain.Address
	if len(publicKeyBytes) >= 20 {
		copy(addr[:], publicKeyBytes[len(publicKeyBytes)-20:])
	}

	return privateKeyBytes, addr, nil
}
