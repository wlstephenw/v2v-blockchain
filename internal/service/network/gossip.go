package network

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

const (
	// TopicBlocks is the pubsub topic for block broadcasting
	TopicBlocks = "v2v-blocks"

	// TopicTransactions is the pubsub topic for transaction broadcasting
	TopicTransactions = "v2v-transactions"

	// TopicConsensus is the pubsub topic for consensus messages
	TopicConsensus = "v2v-consensus"
)

// ConsensusMessageHandler is called when a consensus message is received
type ConsensusMessageHandler func(data []byte) error

// GossipService manages pubsub topics for blockchain message propagation
type GossipService struct {
	host             *Host
	ps               *pubsub.PubSub
	topics           map[string]*pubsub.Topic
	subs             map[string]*pubsub.Subscription
	logger           *logger.Logger
	ctx              context.Context
	cancel           context.CancelFunc
	mu               sync.RWMutex
	consensusHandler ConsensusMessageHandler
}

// BlockMessage represents a block broadcast message
type BlockMessage struct {
	Block     *blockchain.Block `json:"block"`
	PeerID    string            `json:"peer_id"`
	Timestamp int64             `json:"timestamp"`
}

// TransactionMessage represents a transaction broadcast message
type TransactionMessage struct {
	Transaction *blockchain.Transaction `json:"transaction"`
	PeerID      string                  `json:"peer_id"`
	Timestamp   int64                   `json:"timestamp"`
}

// NewGossipService creates a new gossipsub service
func NewGossipService(h *Host, log *logger.Logger) (*GossipService, error) {
	ctx, cancel := context.WithCancel(h.ctx)

	// Create pubsub options
	opts := []pubsub.Option{
		pubsub.WithMessageSignaturePolicy(pubsub.StrictSign),
	}

	// Create pubsub service
	ps, err := pubsub.NewGossipSub(ctx, h.host, opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create pubsub: %w", err)
	}

	return &GossipService{
		host:   h,
		ps:     ps,
		topics: make(map[string]*pubsub.Topic),
		subs:   make(map[string]*pubsub.Subscription),
		logger: log,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start initializes and subscribes to all required topics
func (g *GossipService) Start() error {
	g.logger.Info("Starting gossip service")

	// Subscribe to block topic
	if err := g.SubscribeTopic(TopicBlocks, g.handleBlockMessage); err != nil {
		return fmt.Errorf("failed to subscribe to blocks topic: %w", err)
	}

	// Subscribe to transaction topic
	if err := g.SubscribeTopic(TopicTransactions, g.handleTransactionMessage); err != nil {
		return fmt.Errorf("failed to subscribe to transactions topic: %w", err)
	}

	// Subscribe to consensus topic
	if err := g.SubscribeTopic(TopicConsensus, g.handleConsensusMessage); err != nil {
		return fmt.Errorf("failed to subscribe to consensus topic: %w", err)
	}

	g.logger.Info("Gossip service started",
		logger.Int("topics", len(g.topics)),
	)
	return nil
}

// Stop stops the gossip service
func (g *GossipService) Stop() error {
	g.logger.Info("Stopping gossip service")
	g.cancel()

	g.mu.Lock()
	defer g.mu.Unlock()

	// Unsubscribe from all topics
	for name, sub := range g.subs {
		sub.Cancel()
		g.logger.Debug("Unsubscribed from topic", logger.String("topic", name))
	}

	// Close all topics
	for name, topic := range g.topics {
		topic.Close()
		g.logger.Debug("Closed topic", logger.String("topic", name))
	}

	g.logger.Info("Gossip service stopped")
	return nil
}

// SubscribeTopic subscribes to a topic with a message handler
func (g *GossipService) SubscribeTopic(topicName string, handler func([]byte)) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if already subscribed
	if _, exists := g.topics[topicName]; exists {
		return nil
	}

	// Join the topic
	topic, err := g.ps.Join(topicName)
	if err != nil {
		return fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}

	// Subscribe to the topic
	sub, err := topic.Subscribe()
	if err != nil {
		topic.Close()
		return fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
	}

	g.topics[topicName] = topic
	g.subs[topicName] = sub

	// Start message handler goroutine
	go g.messageHandler(sub, handler)

	g.logger.Info("Subscribed to topic", logger.String("topic", topicName))
	return nil
}

// PublishBlock publishes a block to the network
func (g *GossipService) PublishBlock(block *blockchain.Block) error {
	g.mu.RLock()
	topic, exists := g.topics[TopicBlocks]
	g.mu.RUnlock()

	if !exists {
		return fmt.Errorf("not subscribed to blocks topic")
	}

	msg := BlockMessage{
		Block:     block,
		PeerID:    g.host.ID().String(),
		Timestamp: block.Header.Timestamp,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal block message: %w", err)
	}

	if err := topic.Publish(g.ctx, data); err != nil {
		return fmt.Errorf("failed to publish block: %w", err)
	}

	g.logger.Debug("Published block",
		logger.String("hash", block.Header.Hash.String()),
		logger.Uint64("height", block.Header.Height),
	)
	return nil
}

// PublishTransaction publishes a transaction to the network
func (g *GossipService) PublishTransaction(tx *blockchain.Transaction) error {
	g.mu.RLock()
	topic, exists := g.topics[TopicTransactions]
	g.mu.RUnlock()

	if !exists {
		return fmt.Errorf("not subscribed to transactions topic")
	}

	msg := TransactionMessage{
		Transaction: tx,
		PeerID:      g.host.ID().String(),
		Timestamp:   tx.Timestamp,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction message: %w", err)
	}

	if err := topic.Publish(g.ctx, data); err != nil {
		return fmt.Errorf("failed to publish transaction: %w", err)
	}

	g.logger.Debug("Published transaction", logger.String("hash", tx.Hash.String()))
	return nil
}

// PublishConsensusMessage publishes a consensus message to the network
func (g *GossipService) PublishConsensusMessage(data []byte) error {
	g.mu.RLock()
	topic, exists := g.topics[TopicConsensus]
	g.mu.RUnlock()

	if !exists {
		return fmt.Errorf("not subscribed to consensus topic")
	}

	if err := topic.Publish(g.ctx, data); err != nil {
		return fmt.Errorf("failed to publish consensus message: %w", err)
	}

	g.logger.Debug("Published consensus message")
	return nil
}

// messageHandler handles incoming messages from a subscription
func (g *GossipService) messageHandler(sub *pubsub.Subscription, handler func([]byte)) {
	for {
		select {
		case <-g.ctx.Done():
			return
		default:
		}

		msg, err := sub.Next(g.ctx)
		if err != nil {
			if g.ctx.Err() != nil {
				return
			}
			g.logger.Debug("Error receiving message", logger.ErrField(err))
			continue
		}

		// Skip messages from self
		if msg.ReceivedFrom == g.host.ID() {
			continue
		}

		// Handle the message
		handler(msg.Data)
	}
}

// handleBlockMessage handles incoming block messages
func (g *GossipService) handleBlockMessage(data []byte) {
	var msg BlockMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		g.logger.Debug("Failed to unmarshal block message", logger.ErrField(err))
		return
	}

	g.logger.Debug("Received block message",
		logger.String("hash", msg.Block.Header.Hash.String()),
		logger.Uint64("height", msg.Block.Header.Height),
		logger.String("from", msg.PeerID),
	)

	// TODO: Validate and process the block
	// This will be implemented when integrating with the blockchain package
}

// handleTransactionMessage handles incoming transaction messages
func (g *GossipService) handleTransactionMessage(data []byte) {
	var msg TransactionMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		g.logger.Debug("Failed to unmarshal transaction message", logger.ErrField(err))
		return
	}

	g.logger.Debug("Received transaction message",
		logger.String("hash", msg.Transaction.Hash.String()),
		logger.String("from", msg.PeerID),
	)

	// TODO: Validate and process the transaction
	// This will be implemented when integrating with the transaction pool
}

// SetConsensusHandler sets the handler for consensus messages
func (g *GossipService) SetConsensusHandler(handler ConsensusMessageHandler) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.consensusHandler = handler
	g.logger.Info("Consensus message handler registered")
}

// handleConsensusMessage handles incoming consensus messages
func (g *GossipService) handleConsensusMessage(data []byte) {
	g.mu.RLock()
	handler := g.consensusHandler
	g.mu.RUnlock()

	g.logger.Debug("Received consensus message", logger.Int("size", len(data)))

	if handler != nil {
		if err := handler(data); err != nil {
			g.logger.Debug("Failed to handle consensus message", logger.ErrField(err))
		}
	}
}

// GetPeerCount returns the number of peers in the pubsub mesh for a topic
func (g *GossipService) GetPeerCount(topicName string) int {
	g.mu.RLock()
	topic, exists := g.topics[topicName]
	g.mu.RUnlock()

	if !exists {
		return 0
	}

	return len(topic.ListPeers())
}

