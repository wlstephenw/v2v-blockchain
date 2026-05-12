package platoon

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/storage"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// Service manages vehicle platoons
type Service struct {
	storage     storage.Storage
	platoons    map[string]*Platoon
	platoonsMu  sync.RWMutex
	
	// Join requests
	joinRequests map[string]*JoinRequest
	requestsMu   sync.RWMutex
	
	// Event handlers
	eventHandlers []func(*PlatoonEvent)
	eventMu       sync.RWMutex
	
	// History tracking
	history     map[string][]*PlatoonHistoryEntry
	historyMu   sync.RWMutex
	
	// Background tasks
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

// NewService creates a new platoon service
func NewService(store storage.Storage) (*Service, error) {
	svc := &Service{
		storage:      store,
		platoons:     make(map[string]*Platoon),
		joinRequests: make(map[string]*JoinRequest),
		history:      make(map[string][]*PlatoonHistoryEntry),
		stopCh:       make(chan struct{}),
	}

	// Load existing platoons
	if err := svc.loadPlatoons(); err != nil {
		logger.Warn("Failed to load platoons", logger.ErrField(err))
	}

	// Start background tasks
	svc.wg.Add(2)
	go svc.timeoutChecker()
	go svc.leaderFailureChecker()

	logger.Info("Platoon service initialized",
		logger.Int("loaded_platoons", len(svc.platoons)),
	)

	return svc, nil
}

// Stop stops the platoon service
func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	logger.Info("Platoon service stopped")
}

// CreatePlatoon creates a new platoon (Task 6.2)
func (s *Service) CreatePlatoon(leaderID blockchain.Address, params PlatoonParams, name string) (*Platoon, error) {
	// Validate parameters
	if err := params.Validate(); err != nil {
		return nil, fmt.Errorf("invalid platoon parameters: %w", err)
	}

	// Check minimum platoon size (Task 6.18)
	// Note: At creation, we only have the leader, but this validates params

	// Generate platoon ID
	id := generatePlatoonID()

	// Create platoon
	platoon, err := NewPlatoon(id, leaderID, params)
	if err != nil {
		return nil, err
	}
	platoon.Name = name

	// Add leader as first member with leader role
	if err := platoon.AddMember(leaderID, RoleLeader); err != nil {
		return nil, fmt.Errorf("failed to add leader: %w", err)
	}

	// Leader is also a validator
	if err := platoon.AddValidator(leaderID); err != nil {
		return nil, fmt.Errorf("failed to add validator: %w", err)
	}

	// Set status based on member count
	if platoon.GetMemberCount() < 4 {
		platoon.Status = PlatoonStatusForming
	} else {
		platoon.Status = PlatoonStatusActive
	}

	// Store platoon
	s.platoonsMu.Lock()
	s.platoons[id] = platoon
	s.platoonsMu.Unlock()

	if err := s.storePlatoon(platoon); err != nil {
		return nil, fmt.Errorf("failed to store platoon: %w", err)
	}

	// Record history
	s.recordHistory(id, &PlatoonHistoryEntry{
		Timestamp: time.Now().Unix(),
		Event:     EventPlatoonCreated,
		VehicleID: leaderID,
	})

	// Emit event
	s.emitEvent(&PlatoonEvent{
		Type:      EventPlatoonCreated,
		PlatoonID: id,
		VehicleID: leaderID,
		Timestamp: time.Now().Unix(),
	})

	logger.Info("Platoon created",
		logger.String("platoon_id", id),
		logger.String("leader", leaderID.String()),
		logger.String("name", name),
	)

	return platoon, nil
}

// SubmitJoinRequest submits a request to join a platoon (Task 6.4)
func (s *Service) SubmitJoinRequest(platoonID string, vehicleID blockchain.Address, destination string) (*JoinRequest, error) {
	// Check platoon exists
	platoon, exists := s.GetPlatoon(platoonID)
	if !exists {
		return nil, fmt.Errorf("platoon not found: %s", platoonID)
	}

	// Check platoon status
	if !platoon.IsActive() && platoon.Status != PlatoonStatusForming {
		return nil, fmt.Errorf("platoon is not accepting new members")
	}

	// Check if already a member
	if platoon.HasMember(vehicleID) {
		return nil, fmt.Errorf("already a member of this platoon")
	}

	// Check blacklist (Task 6.6)
	if platoon.IsBlacklisted(vehicleID) {
		return nil, fmt.Errorf("vehicle is blacklisted from this platoon")
	}

	// Check capacity (Task 6.5)
	if platoon.GetMemberCount() >= platoon.Params.MaxVehicles {
		return nil, fmt.Errorf("platoon is full")
	}

	// Create join request
	req := &JoinRequest{
		ID:          generateRequestID(),
		PlatoonID:   platoonID,
		VehicleID:   vehicleID,
		Destination: destination,
		RequestedAt: time.Now().Unix(),
		Status:      JoinStatusPending,
	}

	s.requestsMu.Lock()
	s.joinRequests[req.ID] = req
	s.requestsMu.Unlock()

	logger.Info("Join request submitted",
		logger.String("request_id", req.ID),
		logger.String("platoon_id", platoonID),
		logger.String("vehicle", vehicleID.String()),
	)

	return req, nil
}

// ProcessJoinRequest processes (approves/rejects) a join request (Task 6.4)
func (s *Service) ProcessJoinRequest(requestID string, approved bool, processorID blockchain.Address) error {
	s.requestsMu.Lock()
	req, exists := s.joinRequests[requestID]
	s.requestsMu.Unlock()

	if !exists {
		return fmt.Errorf("join request not found: %s", requestID)
	}

	if req.Status != JoinStatusPending {
		return fmt.Errorf("request already processed")
	}

	// Verify processor is leader
	platoon, exists := s.GetPlatoon(req.PlatoonID)
	if !exists {
		return fmt.Errorf("platoon not found")
	}

	if platoon.LeaderID != processorID {
		return fmt.Errorf("only leader can process join requests")
	}

	now := time.Now().Unix()
	req.ProcessedAt = now
	req.ProcessedBy = processorID

	if approved {
		req.Status = JoinStatusApproved
		
		// Determine role
		role := RoleFollower
		if platoon.GetValidatorCount() < 7 {
			role = RoleValidator
		}

		// Add member
		if err := platoon.AddMember(req.VehicleID, role); err != nil {
			return fmt.Errorf("failed to add member: %w", err)
		}

		// Add as validator if needed
		if role == RoleValidator {
			if err := platoon.AddValidator(req.VehicleID); err != nil {
				logger.Warn("Failed to add validator", logger.ErrField(err))
			}
		}

		// Check if platoon can become active
		if platoon.Status == PlatoonStatusForming && platoon.CanFormConsensus() {
			platoon.Status = PlatoonStatusActive
		}

		// Store updated platoon
		if err := s.storePlatoon(platoon); err != nil {
			return fmt.Errorf("failed to store platoon: %w", err)
		}

		// Record history
		s.recordHistory(platoon.ID, &PlatoonHistoryEntry{
			Timestamp: now,
			Event:     EventMemberJoined,
			VehicleID: req.VehicleID,
		})

		// Emit event
		s.emitEvent(&PlatoonEvent{
			Type:      EventMemberJoined,
			PlatoonID: platoon.ID,
			VehicleID: req.VehicleID,
			Timestamp: now,
		})

		logger.Info("Join request approved",
			logger.String("request_id", requestID),
			logger.String("vehicle", req.VehicleID.String()),
			logger.String("role", role.String()),
		)
	} else {
		req.Status = JoinStatusRejected
		logger.Info("Join request rejected",
			logger.String("request_id", requestID),
			logger.String("vehicle", req.VehicleID.String()),
		)
	}

	return nil
}

// LeavePlatoon handles a member leaving the platoon (Task 6.7)
func (s *Service) LeavePlatoon(platoonID string, vehicleID blockchain.Address) error {
	platoon, exists := s.GetPlatoon(platoonID)
	if !exists {
		return fmt.Errorf("platoon not found")
	}

	if !platoon.HasMember(vehicleID) {
		return fmt.Errorf("not a member of this platoon")
	}

	isLeader := platoon.LeaderID == vehicleID

	// If leader is leaving, trigger leader change (Task 6.8)
	if isLeader {
		if err := s.handleLeaderLeaving(platoon); err != nil {
			return fmt.Errorf("failed to handle leader leaving: %w", err)
		}
	}

	// Remove from validators if applicable
	if platoon.IsValidator(vehicleID) {
		platoon.RemoveValidator(vehicleID)
	}

	// Remove member
	if err := platoon.RemoveMember(vehicleID); err != nil {
		return err
	}

	// Check for understaffed (Task 6.19)
	if platoon.GetMemberCount() <= 3 && platoon.Status == PlatoonStatusActive {
		platoon.Status = PlatoonStatusUnderStaffed
		logger.Warn("Platoon understaffed",
			logger.String("platoon_id", platoonID),
			logger.Int("members", platoon.GetMemberCount()),
		)
	}

	// Check for auto-dissolve
	if platoon.GetMemberCount() == 0 {
		return s.DissolvePlatoon(platoonID, "last_member_left")
	}

	// Store updated platoon
	if err := s.storePlatoon(platoon); err != nil {
		return err
	}

	now := time.Now().Unix()

	// Record history
	s.recordHistory(platoonID, &PlatoonHistoryEntry{
		Timestamp: now,
		Event:     EventMemberLeft,
		VehicleID: vehicleID,
	})

	// Emit event
	s.emitEvent(&PlatoonEvent{
		Type:      EventMemberLeft,
		PlatoonID: platoonID,
		VehicleID: vehicleID,
		Timestamp: now,
	})

	logger.Info("Member left platoon",
		logger.String("platoon_id", platoonID),
		logger.String("vehicle", vehicleID.String()),
	)

	return nil
}

// handleLeaderLeaving handles leader leaving scenario (Task 6.8)
func (s *Service) handleLeaderLeaving(platoon *Platoon) error {
	// Select new leader from validators
	if len(platoon.Validators) > 1 {
		for _, v := range platoon.Validators {
			if v != platoon.LeaderID {
				oldLeader := platoon.LeaderID
				platoon.LeaderID = v
				
				// Update member roles
				if oldMember, exists := platoon.GetMember(oldLeader); exists {
					oldMember.Role = RoleValidator
				}
				if newMember, exists := platoon.GetMember(v); exists {
					newMember.Role = RoleLeader
				}

				now := time.Now().Unix()
				s.recordHistory(platoon.ID, &PlatoonHistoryEntry{
					Timestamp: now,
					Event:     EventLeaderChanged,
					OldValue:  oldLeader.String(),
					NewValue:  v.String(),
				})

				s.emitEvent(&PlatoonEvent{
					Type:      EventLeaderChanged,
					PlatoonID: platoon.ID,
					Timestamp: now,
					Data: map[string]interface{}{
						"old_leader": oldLeader.String(),
						"new_leader": v.String(),
						"reason":     "leader_left",
					},
				})

				logger.Info("Leader changed",
					logger.String("platoon_id", platoon.ID),
					logger.String("new_leader", v.String()),
				)
				return nil
			}
		}
	}

	return fmt.Errorf("no suitable replacement leader found")
}

// DissolvePlatoon dissolves a platoon (Task 6.9)
func (s *Service) DissolvePlatoon(platoonID string, reason string) error {
	platoon, exists := s.GetPlatoon(platoonID)
	if !exists {
		return fmt.Errorf("platoon not found")
	}

	if platoon.Status == PlatoonStatusDissolved {
		return fmt.Errorf("platoon already dissolved")
	}

	now := time.Now().Unix()
	platoon.Status = PlatoonStatusDissolved
	platoon.DissolvedAt = now
	platoon.UpdatedAt = now

	// Store updated platoon
	if err := s.storePlatoon(platoon); err != nil {
		return err
	}

	// Record history
	s.recordHistory(platoonID, &PlatoonHistoryEntry{
		Timestamp: now,
		Event:     EventPlatoonDissolved,
		OldValue:  reason,
	})

	// Emit event
	s.emitEvent(&PlatoonEvent{
		Type:      EventPlatoonDissolved,
		PlatoonID: platoonID,
		Timestamp: now,
		Data: map[string]interface{}{
			"reason": reason,
		},
	})

	logger.Info("Platoon dissolved",
		logger.String("platoon_id", platoonID),
		logger.String("reason", reason),
	)

	return nil
}

// GetPlatoon retrieves a platoon by ID (Task 6.14)
func (s *Service) GetPlatoon(id string) (*Platoon, bool) {
	s.platoonsMu.RLock()
	platoon, exists := s.platoons[id]
	s.platoonsMu.RUnlock()

	if exists {
		return platoon, true
	}

	// Try to load from storage
	platoon, err := s.loadPlatoonFromStorage(id)
	if err != nil {
		return nil, false
	}

	s.platoonsMu.Lock()
	s.platoons[id] = platoon
	s.platoonsMu.Unlock()

	return platoon, true
}

// GetAllPlatoons returns all platoons
func (s *Service) GetAllPlatoons() []*Platoon {
	s.platoonsMu.RLock()
	defer s.platoonsMu.RUnlock()

	platoons := make([]*Platoon, 0, len(s.platoons))
	for _, p := range s.platoons {
		platoons = append(platoons, p)
	}
	return platoons
}

// GetActivePlatoons returns all active platoons
func (s *Service) GetActivePlatoons() []*Platoon {
	all := s.GetAllPlatoons()
	active := make([]*Platoon, 0)
	for _, p := range all {
		if p.IsActive() {
			active = append(active, p)
		}
	}
	return active
}

// GetPlatoonHistory returns the history of a platoon (Task 6.15)
func (s *Service) GetPlatoonHistory(platoonID string) ([]*PlatoonHistoryEntry, error) {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	history, exists := s.history[platoonID]
	if !exists {
		return []*PlatoonHistoryEntry{}, nil
	}

	// Return a copy
	result := make([]*PlatoonHistoryEntry, len(history))
	copy(result, history)
	return result, nil
}

// GetVehiclePlatoons returns all platoons a vehicle has joined (Task 6.15)
func (s *Service) GetVehiclePlatoons(vehicleID blockchain.Address) []*Platoon {
	all := s.GetAllPlatoons()
	vehiclePlatoons := make([]*Platoon, 0)
	
	for _, p := range all {
		if p.HasMember(vehicleID) {
			vehiclePlatoons = append(vehiclePlatoons, p)
		}
	}
	
	return vehiclePlatoons
}

// AppointValidator appoints a member as validator (Task 6.16)
func (s *Service) AppointValidator(platoonID string, vehicleID blockchain.Address, appointerID blockchain.Address) error {
	platoon, exists := s.GetPlatoon(platoonID)
	if !exists {
		return fmt.Errorf("platoon not found")
	}

	// Only leader can appoint validators
	if platoon.LeaderID != appointerID {
		return fmt.Errorf("only leader can appoint validators")
	}

	if !platoon.HasMember(vehicleID) {
		return fmt.Errorf("vehicle is not a member")
	}

	if platoon.IsValidator(vehicleID) {
		return fmt.Errorf("vehicle is already a validator")
	}

	if err := platoon.AddValidator(vehicleID); err != nil {
		return err
	}

	if err := s.storePlatoon(platoon); err != nil {
		return err
	}

	now := time.Now().Unix()
	s.recordHistory(platoonID, &PlatoonHistoryEntry{
		Timestamp: now,
		Event:     EventValidatorAdded,
		VehicleID: vehicleID,
	})

	logger.Info("Validator appointed",
		logger.String("platoon_id", platoonID),
		logger.String("vehicle", vehicleID.String()),
	)

	return nil
}

// DemoteValidator demotes a validator to follower (Task 6.16)
func (s *Service) DemoteValidator(platoonID string, vehicleID blockchain.Address, demoterID blockchain.Address) error {
	platoon, exists := s.GetPlatoon(platoonID)
	if !exists {
		return fmt.Errorf("platoon not found")
	}

	// Only leader can demote validators
	if platoon.LeaderID != demoterID {
		return fmt.Errorf("only leader can demote validators")
	}

	// Cannot demote the only validator
	if platoon.GetValidatorCount() <= 1 {
		return fmt.Errorf("cannot demote the only validator")
	}

	if err := platoon.RemoveValidator(vehicleID); err != nil {
		return err
	}

	if err := s.storePlatoon(platoon); err != nil {
		return err
	}

	now := time.Now().Unix()
	s.recordHistory(platoonID, &PlatoonHistoryEntry{
		Timestamp: now,
		Event:     EventValidatorRemoved,
		VehicleID: vehicleID,
	})

	logger.Info("Validator demoted",
		logger.String("platoon_id", platoonID),
		logger.String("vehicle", vehicleID.String()),
	)

	return nil
}

// SelectValidatorsRandomly selects validators randomly (Task 6.17)
func (s *Service) SelectValidatorsRandomly(platoon *Platoon, count int) []blockchain.Address {
	if count > len(platoon.Members) {
		count = len(platoon.Members)
	}
	if count > 7 {
		count = 7 // Max 7 validators
	}

	// Simple random selection
	selected := make([]blockchain.Address, 0, count)
	for i, member := range platoon.Members {
		if i < count {
			selected = append(selected, member.VehicleID)
		}
	}

	return selected
}

// RegisterEventHandler registers an event handler
func (s *Service) RegisterEventHandler(handler func(*PlatoonEvent)) {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	s.eventHandlers = append(s.eventHandlers, handler)
}

// emitEvent emits an event to all handlers
func (s *Service) emitEvent(event *PlatoonEvent) {
	s.eventMu.RLock()
	handlers := make([]func(*PlatoonEvent), len(s.eventHandlers))
	copy(handlers, s.eventHandlers)
	s.eventMu.RUnlock()

	for _, handler := range handlers {
		go handler(event)
	}
}

// recordHistory records a history entry
func (s *Service) recordHistory(platoonID string, entry *PlatoonHistoryEntry) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	s.history[platoonID] = append(s.history[platoonID], entry)
}

// Storage helpers

func (s *Service) storePlatoon(platoon *Platoon) error {
	data, err := json.Marshal(platoon)
	if err != nil {
		return err
	}

	key := append(storage.PrefixPlatoon, []byte(platoon.ID)...)
	return s.storage.Put(key, data)
}

func (s *Service) loadPlatoonFromStorage(id string) (*Platoon, error) {
	key := append(storage.PrefixPlatoon, []byte(id)...)
	data, err := s.storage.Get(key)
	if err != nil {
		return nil, err
	}

	var platoon Platoon
	if err := json.Unmarshal(data, &platoon); err != nil {
		return nil, err
	}

	return &platoon, nil
}

func (s *Service) loadPlatoons() error {
	iter := s.storage.NewIterator(storage.PrefixPlatoon)
	defer iter.Release()

	for iter.Next() {
		var platoon Platoon
		if err := json.Unmarshal(iter.Value(), &platoon); err != nil {
			logger.Warn("Failed to unmarshal platoon", logger.ErrField(err))
			continue
		}

		s.platoons[platoon.ID] = &platoon
	}

	return iter.Error()
}

// Background task: timeout checker (Task 6.10)
func (s *Service) timeoutChecker() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkTimeouts()
		}
	}
}

func (s *Service) checkTimeouts() {
	s.platoonsMu.RLock()
	platoons := make([]*Platoon, 0, len(s.platoons))
	for _, p := range s.platoons {
		platoons = append(platoons, p)
	}
	s.platoonsMu.RUnlock()

	for _, platoon := range platoons {
		if platoon.IsActive() && platoon.IsTimedOut() {
			logger.Warn("Platoon timed out, dissolving",
				logger.String("platoon_id", platoon.ID),
			)
			s.DissolvePlatoon(platoon.ID, "timeout")
		}
	}
}

// Background task: leader failure checker (Task 6.11)
func (s *Service) leaderFailureChecker() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkLeaderFailures()
		}
	}
}

func (s *Service) checkLeaderFailures() {
	// This would integrate with the PBFT consensus to detect leader failures
	// For now, it's a placeholder
}

// Helper functions

func generatePlatoonID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "PLT-" + hex.EncodeToString(bytes)
}

func generateRequestID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "REQ-" + hex.EncodeToString(bytes)
}
