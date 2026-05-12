package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/consensus"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/executor"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/identity"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/message"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/platoon"
	"github.com/v2v-blockchain/v2v-blockchain/internal/service/state"
	"github.com/v2v-blockchain/v2v-blockchain/internal/infra/storage"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/txpool"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// ServerConfig holds API server configuration
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultServerConfig returns default configuration
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:         "0.0.0.0",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

// Server represents the API server
type Server struct {
	config    ServerConfig
	router    *mux.Router
	server    *http.Server
	
	// Services
	bc        *blockchain.Blockchain
	storage   storage.Storage
	txPool    *txpool.TxPool
	identity  *identity.Service
	platoon   *platoon.Service
	consensus *consensus.PBFTEngine
	state     *state.Service
	message   *message.Service

	// Transaction builder
	txBuilder *executor.TransactionBuilder
	
	// WebSocket
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
	broadcast chan interface{}
	
	// Lifecycle
	stopCh    chan struct{}
}

// NewServer creates a new API server
func NewServer(
	config ServerConfig,
	bc *blockchain.Blockchain,
	store storage.Storage,
	txPool *txpool.TxPool,
	idService *identity.Service,
	platoonService *platoon.Service,
	consensusEngine *consensus.PBFTEngine,
	stateService *state.Service,
	msgService *message.Service,
) *Server {
	s := &Server{
		config:    config,
		router:    mux.NewRouter(),
		bc:        bc,
		storage:   store,
		txPool:    txPool,
		identity:  idService,
		platoon:   platoonService,
		consensus: consensusEngine,
		state:     stateService,
		message:   msgService,
		txBuilder: executor.NewTransactionBuilder(),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
		clients:   make(map[*websocket.Conn]bool),
		broadcast: make(chan interface{}, 100),
		stopCh:    make(chan struct{}),
	}

	s.setupRoutes()
	s.setupMiddleware()

	return s
}

// setupRoutes configures API routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/ready", s.handleReady).Methods("GET")

	// Block APIs (Task 9.6)
	s.router.HandleFunc("/api/v1/blocks/latest", s.handleGetLatestBlock).Methods("GET")
	s.router.HandleFunc("/api/v1/blocks/{height}", s.handleGetBlockByHeight).Methods("GET")
	s.router.HandleFunc("/api/v1/blocks/hash/{hash}", s.handleGetBlockByHash).Methods("GET")
	s.router.HandleFunc("/api/v1/blocks", s.handleGetBlocks).Methods("GET")

	// Transaction APIs (Task 9.5, 9.7)
	s.router.HandleFunc("/api/v1/transactions", s.handleSubmitTransaction).Methods("POST")
	s.router.HandleFunc("/api/v1/transactions/{hash}", s.handleGetTransaction).Methods("GET")
	s.router.HandleFunc("/api/v1/transactions/pending", s.handleGetPendingTransactions).Methods("GET")

	// Platoon APIs (Task 9.8)
	s.router.HandleFunc("/api/v1/platoons", s.handleGetPlatoons).Methods("GET")
	s.router.HandleFunc("/api/v1/platoons", s.handleCreatePlatoon).Methods("POST")
	s.router.HandleFunc("/api/v1/platoons/{id}", s.handleGetPlatoon).Methods("GET")
	s.router.HandleFunc("/api/v1/platoons/{id}/join", s.handleJoinPlatoon).Methods("POST")
	s.router.HandleFunc("/api/v1/platoons/{id}/leave", s.handleLeavePlatoon).Methods("POST")
	s.router.HandleFunc("/api/v1/platoons/{id}/dissolve", s.handleDissolvePlatoon).Methods("POST")
	s.router.HandleFunc("/api/v1/platoons/{id}/members", s.handleGetPlatoonMembers).Methods("GET")
	s.router.HandleFunc("/api/v1/platoons/{id}/history", s.handleGetPlatoonHistory).Methods("GET")

	// Identity APIs
	s.router.HandleFunc("/api/v1/identities/{id}", s.handleGetIdentity).Methods("GET")
	s.router.HandleFunc("/api/v1/identities", s.handleListIdentities).Methods("GET")

	// Node status API (Task 9.9)
	s.router.HandleFunc("/api/v1/node/status", s.handleNodeStatus).Methods("GET")
	s.router.HandleFunc("/api/v1/node/peers", s.handleNodePeers).Methods("GET")
	s.router.HandleFunc("/api/v1/node/stats", s.handleNodeStats).Methods("GET")

	// State/History APIs
	s.router.HandleFunc("/api/v1/state/{entity}/events", s.handleGetStateEvents).Methods("GET")
	s.router.HandleFunc("/api/v1/state/{entity}/history", s.handleGetStateHistory).Methods("GET")
	s.router.HandleFunc("/api/v1/audit/logs", s.handleGetAuditLogs).Methods("GET")

	// WebSocket (Task 9.10)
	s.router.HandleFunc("/ws", s.handleWebSocket)
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Logging middleware
	s.router.Use(s.loggingMiddleware)
	
	// Recovery middleware
	s.router.Use(s.recoveryMiddleware)
	
	// CORS middleware
	s.router.Use(s.corsMiddleware)
}

// Start starts the API server
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	
	s.server = &http.Server{
		Addr:         addr,
		Handler:      s.router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	// Start WebSocket broadcaster
	go s.websocketBroadcaster()

	logger.Info("API server starting", logger.String("addr", addr))
	
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", logger.ErrField(err))
		}
	}()

	return nil
}

// Stop stops the API server
func (s *Server) Stop() error {
	close(s.stopCh)
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	return s.server.Shutdown(ctx)
}

// === Health Check Handlers ===

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ready := s.consensus != nil
	if ready {
		s.jsonResponse(w, http.StatusOK, map[string]string{
			"status": "ready",
		})
	} else {
		s.jsonResponse(w, http.StatusServiceUnavailable, map[string]string{
			"status": "not ready",
		})
	}
}

// === Block API Handlers (Task 9.6) ===

func (s *Server) handleGetLatestBlock(w http.ResponseWriter, r *http.Request) {
	block, err := s.bc.GetLatestBlock()
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}
	s.jsonResponse(w, http.StatusOK, block)
}

func (s *Server) handleGetBlockByHeight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	height, err := strconv.ParseUint(vars["height"], 10, 64)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid height")
		return
	}

	block, err := s.bc.GetBlockByHeight(height)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, block)
}

func (s *Server) handleGetBlockByHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hashStr := vars["hash"]
	
	hash, err := blockchain.HexToHash(hashStr)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid hash")
		return
	}

	block, err := s.bc.GetBlockByHash(hash)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, block)
}

func (s *Server) handleGetBlocks(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset, _ := strconv.Atoi(offsetStr)
	if offset < 0 {
		offset = 0
	}

	latest, err := s.bc.GetLatestBlock()
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	end := offset + limit
	if end > int(latest.Header.Height) {
		end = int(latest.Header.Height)
	}

	blocks, err := s.bc.GetBlocksRange(uint64(offset), uint64(end))
	if err != nil {
		s.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"blocks": blocks,
		"limit":  limit,
		"offset": offset,
	})
}

// === Transaction API Handlers (Task 9.5, 9.7) ===

func (s *Server) handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
	var tx blockchain.Transaction
	if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid transaction format")
		return
	}

	// Validate and add to pool
	if err := s.txPool.AddTx(&tx, 5); err != nil {
		s.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"hash":   tx.Hash.String(),
		"status": "pending",
	})
}

func (s *Server) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hashStr := vars["hash"]

	hash, err := blockchain.HexToHash(hashStr)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid hash")
		return
	}

	// Check pending pool first
	if tx, exists := s.txPool.GetTx(hash); exists {
		s.jsonResponse(w, http.StatusOK, map[string]interface{}{
			"transaction": tx,
			"status":      "pending",
		})
		return
	}

	// Check blockchain
	tx, err := s.bc.GetTransaction(hash)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"transaction": tx,
		"status":      "confirmed",
	})
}

func (s *Server) handleGetPendingTransactions(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	txs := s.txPool.GetPending(limit)
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"transactions": txs,
		"count":        len(txs),
	})
}

// === Platoon API Handlers (Task 9.8) ===

func (s *Server) handleGetPlatoons(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	
	platoons := s.platoon.GetAllPlatoons()
	
	// Filter by status if specified
	if status != "" {
		filtered := make([]*platoon.Platoon, 0)
		for _, p := range platoons {
			if string(p.Status) == status {
				filtered = append(filtered, p)
			}
		}
		platoons = filtered
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"platoons": platoons,
		"count":    len(platoons),
	})
}

func (s *Server) handleGetPlatoon(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	platoonID := vars["id"]

	p, exists := s.platoon.GetPlatoon(platoonID)
	if !exists {
		s.errorResponse(w, http.StatusNotFound, "platoon not found")
		return
	}

	s.jsonResponse(w, http.StatusOK, p)
}

func (s *Server) handleGetPlatoonMembers(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	platoonID := vars["id"]

	p, exists := s.platoon.GetPlatoon(platoonID)
	if !exists {
		s.errorResponse(w, http.StatusNotFound, "platoon not found")
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"members": p.Members,
		"count":   len(p.Members),
	})
}

func (s *Server) handleGetPlatoonHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	platoonID := vars["id"]

	history, err := s.platoon.GetPlatoonHistory(platoonID)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"history": history,
	})
}

// === Identity API Handlers ===

func (s *Server) handleGetIdentity(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := blockchain.HexToAddress(idStr)
	if err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid address")
		return
	}

	identity, exists := s.identity.GetIdentity(id)
	if !exists {
		s.errorResponse(w, http.StatusNotFound, "identity not found")
		return
	}

	s.jsonResponse(w, http.StatusOK, identity)
}

func (s *Server) handleListIdentities(w http.ResponseWriter, r *http.Request) {
	// ListIdentities not implemented yet
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"identities": []string{},
		"count":      0,
		"note":       "list identities not implemented",
	})
}

// === Node Status API Handlers (Task 9.9) ===

func (s *Server) handleNodeStatus(w http.ResponseWriter, r *http.Request) {
	latest, _ := s.bc.GetLatestBlock()
	status := map[string]interface{}{
		"version": "1.0.0",
		"chain": map[string]interface{}{
			"height": latest.Header.Height,
			"hash":   latest.Header.Hash.String(),
		},
	}

	s.jsonResponse(w, http.StatusOK, status)
}

func (s *Server) handleNodePeers(w http.ResponseWriter, r *http.Request) {
	// Network module not implemented yet
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"peers": []string{},
		"count": 0,
		"note":  "network module not implemented",
	})
}

func (s *Server) handleNodeStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"txpool": s.txPool.GetStats(),
	}

	s.jsonResponse(w, http.StatusOK, stats)
}

// === State/History API Handlers ===

func (s *Server) handleGetStateEvents(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	entityID := vars["entity"]

	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	events := s.state.GetEvents(entityID, limit)
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"events": events,
		"count":  len(events),
	})
}

func (s *Server) handleGetStateHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	entityID := vars["entity"]

	timestampStr := r.URL.Query().Get("timestamp")
	timestamp, _ := strconv.ParseInt(timestampStr, 10, 64)
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}

	snapshot, err := s.state.GetStateAtTime(entityID, timestamp)
	if err != nil {
		s.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusOK, snapshot)
}

func (s *Server) handleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	// GetAuditLogs not implemented yet
	s.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"logs":  []string{},
		"count": 0,
		"note":  "audit logs not implemented",
	})
}

// === WebSocket Handler (Task 9.10) ===

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Warn("WebSocket upgrade failed", logger.ErrField(err))
		return
	}
	defer conn.Close()

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
	}()

	// Send initial connection message
	conn.WriteJSON(map[string]string{
		"type":    "connected",
		"message": "WebSocket connected",
	})

	// Handle client messages
	for {
		var msg map[string]interface{}
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Warn("WebSocket error", logger.ErrField(err))
			}
			break
		}

		// Handle subscription requests
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "subscribe_blocks":
				conn.WriteJSON(map[string]string{
					"type":    "subscribed",
					"channel": "blocks",
				})
			case "subscribe_transactions":
				conn.WriteJSON(map[string]string{
					"type":    "subscribed",
					"channel": "transactions",
				})
			case "subscribe_platoons":
				conn.WriteJSON(map[string]string{
					"type":    "subscribed",
					"channel": "platoons",
				})
			}
		}
	}
}

// websocketBroadcaster broadcasts messages to all connected clients
func (s *Server) websocketBroadcaster() {
	for {
		select {
		case <-s.stopCh:
			return
		case msg := <-s.broadcast:
			s.clientsMu.RLock()
			clients := make([]*websocket.Conn, 0, len(s.clients))
			for client := range s.clients {
				clients = append(clients, client)
			}
			s.clientsMu.RUnlock()

			for _, client := range clients {
				if err := client.WriteJSON(msg); err != nil {
					// Client disconnected
					s.clientsMu.Lock()
					delete(s.clients, client)
					s.clientsMu.Unlock()
					client.Close()
				}
			}
		}
	}
}

// BroadcastBlock broadcasts a new block to all WebSocket clients
func (s *Server) BroadcastBlock(block *blockchain.Block) {
	select {
	case s.broadcast <- map[string]interface{}{
		"type":  "new_block",
		"block": block,
	}:
	default:
	}
}

// BroadcastTransaction broadcasts a new transaction to all WebSocket clients
func (s *Server) BroadcastTransaction(tx *blockchain.Transaction) {
	select {
	case s.broadcast <- map[string]interface{}{
		"type": "new_transaction",
		"tx":   tx,
	}:
	default:
	}
}

// BroadcastPlatoonEvent broadcasts a platoon event to all WebSocket clients
func (s *Server) BroadcastPlatoonEvent(eventType string, data interface{}) {
	select {
	case s.broadcast <- map[string]interface{}{
		"type": eventType,
		"data": data,
	}:
	default:
	}
}

// === Middleware ===

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Debug("API request",
			logger.String("method", r.Method),
			logger.String("path", r.URL.Path),
			logger.Duration("duration", time.Since(start)),
		)
	})
}

func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("API panic recovered", logger.Any("error", err))
				s.errorResponse(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// === Helper Methods ===

func (s *Server) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) errorResponse(w http.ResponseWriter, status int, message string) {
	s.jsonResponse(w, status, map[string]string{
		"error": message,
	})
}

// === Platoon Transaction Handlers ===

func (s *Server) handleCreatePlatoon(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string               `json:"name"`
		SafeDistance float64              `json:"safe_distance"`
		MaxSize      int                  `json:"max_size"`
		TargetSpeed  float64              `json:"target_speed"`
		Validators   []blockchain.Address `json:"validators"`
		From         blockchain.Address   `json:"from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request format")
		return
	}

	// Build transaction
	tx := s.txBuilder.BuildCreatePlatoonTx(
		req.From,
		req.Name,
		req.SafeDistance,
		req.MaxSize,
		req.TargetSpeed,
		req.Validators,
	)

	// Add to transaction pool (high priority for platoon operations)
	if err := s.txPool.AddTx(tx, 10); err != nil {
		s.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"tx_hash": tx.Hash.String(),
		"status":  "pending",
		"note":    "Transaction submitted, will be executed after consensus",
	})
}

func (s *Server) handleJoinPlatoon(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	platoonID := vars["id"]

	var req struct {
		From        blockchain.Address `json:"from"`
		Destination string             `json:"destination"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request format")
		return
	}

	tx := s.txBuilder.BuildJoinPlatoonTx(req.From, platoonID, req.Destination)

	if err := s.txPool.AddTx(tx, 8); err != nil {
		s.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"tx_hash": tx.Hash.String(),
		"status":  "pending",
	})
}

func (s *Server) handleLeavePlatoon(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	platoonID := vars["id"]

	var req struct {
		From blockchain.Address `json:"from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request format")
		return
	}

	tx := s.txBuilder.BuildLeavePlatoonTx(req.From, platoonID)

	if err := s.txPool.AddTx(tx, 8); err != nil {
		s.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"tx_hash": tx.Hash.String(),
		"status":  "pending",
	})
}

func (s *Server) handleDissolvePlatoon(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	platoonID := vars["id"]

	var req struct {
		From blockchain.Address `json:"from"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.errorResponse(w, http.StatusBadRequest, "invalid request format")
		return
	}

	tx := s.txBuilder.BuildDissolvePlatoonTx(req.From, platoonID)

	if err := s.txPool.AddTx(tx, 10); err != nil {
		s.errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	s.jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"tx_hash": tx.Hash.String(),
		"status":  "pending",
	})
}
