package state

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/infra/storage"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// Service manages state traceability
type Service struct {
	storage    storage.Storage
	policy     RetentionPolicy
	
	// Event storage
	events     map[string][]*StateChangeEvent // entity ID -> events
	eventsMu   sync.RWMutex
	
	// Audit log chain
	auditLog   []*AuditLogRecord
	auditMu    sync.RWMutex
	lastHash   blockchain.Hash
	
	// Vehicle history
	vehicleHistory map[blockchain.Address][]*VehiclePlatoonHistory
	historyMu      sync.RWMutex
	
	// Anomalies
	anomalies      []*AnomalyReport
	anomalyMu      sync.RWMutex
	
	// Subscribers
	subscribers    []func(*StateChangeEvent)
	subMu          sync.RWMutex
	
	// Background tasks
	stopCh         chan struct{}
	wg             sync.WaitGroup
}

// NewService creates a new state traceability service
func NewService(store storage.Storage) *Service {
	svc := &Service{
		storage:        store,
		policy:         DefaultRetentionPolicy(),
		events:         make(map[string][]*StateChangeEvent),
		auditLog:       make([]*AuditLogRecord, 0),
		vehicleHistory: make(map[blockchain.Address][]*VehiclePlatoonHistory),
		anomalies:      make([]*AnomalyReport, 0),
		stopCh:         make(chan struct{}),
	}

	// Load existing data
	if err := svc.loadEvents(); err != nil {
		logger.Warn("Failed to load events", logger.ErrField(err))
	}
	if err := svc.loadAuditLog(); err != nil {
		logger.Warn("Failed to load audit log", logger.ErrField(err))
	}

	// Start background tasks
	svc.wg.Add(2)
	go svc.batchAnchorLoop()
	go svc.retentionPolicyLoop()

	logger.Info("State traceability service initialized")
	return svc
}

// Stop stops the state service
func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	logger.Info("State traceability service stopped")
}

// RecordEvent records a state change event (Task 8.2, 8.3, 8.4)
func (s *Service) RecordEvent(event *StateChangeEvent) error {
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid event: %w", err)
	}

	// Store event
	s.eventsMu.Lock()
	s.events[event.EntityID] = append(s.events[event.EntityID], event)
	s.eventsMu.Unlock()

	// Create audit log record (Task 8.8)
	if err := s.createAuditRecord(event); err != nil {
		logger.Warn("Failed to create audit record", logger.ErrField(err))
	}

	// Update vehicle history if applicable
	if event.EntityType == "vehicle" {
		s.updateVehicleHistory(event)
	}

	// Check for anomalies (Task 8.15)
	s.checkForAnomalies(event)

	// Notify subscribers (Task 8.11)
	s.notifySubscribers(event)

	logger.Debug("State event recorded",
		logger.String("type", event.Type.String()),
		logger.String("entity", event.EntityID),
	)

	return nil
}

// RecordPlatoonCreated records platoon creation (Task 8.2)
func (s *Service) RecordPlatoonCreated(platoonID string, creator blockchain.Address, blockHeight uint64, txHash blockchain.Hash) error {
	event := &StateChangeEvent{
		ID:          generateEventID(),
		Type:        ChangeTypePlatoonCreated,
		EntityID:    platoonID,
		EntityType:  "platoon",
		Timestamp:   time.Now().Unix(),
		BlockHeight: blockHeight,
		TxHash:      txHash,
		ActorID:     creator,
		Metadata: map[string]interface{}{
			"creator": creator.String(),
		},
	}
	return s.RecordEvent(event)
}

// RecordMemberJoined records member joining (Task 8.3)
func (s *Service) RecordMemberJoined(platoonID string, vehicleID blockchain.Address, blockHeight uint64, txHash blockchain.Hash) error {
	event := &StateChangeEvent{
		ID:          generateEventID(),
		Type:        ChangeTypeMemberJoined,
		EntityID:    platoonID,
		EntityType:  "platoon",
		Timestamp:   time.Now().Unix(),
		BlockHeight: blockHeight,
		TxHash:      txHash,
		ActorID:     vehicleID,
		Metadata: map[string]interface{}{
			"vehicle_id": vehicleID.String(),
		},
	}
	
	if err := s.RecordEvent(event); err != nil {
		return err
	}

	// Also record for vehicle
	vehicleEvent := &StateChangeEvent{
		ID:          generateEventID(),
		Type:        ChangeTypeMemberJoined,
		EntityID:    vehicleID.String(),
		EntityType:  "vehicle",
		Timestamp:   event.Timestamp,
		BlockHeight: blockHeight,
		TxHash:      txHash,
		ActorID:     vehicleID,
		Metadata: map[string]interface{}{
			"platoon_id": platoonID,
		},
	}
	return s.RecordEvent(vehicleEvent)
}

// RecordStatusChange records a status change with old/new values (Task 8.4)
func (s *Service) RecordStatusChange(
	entityID string,
	entityType string,
	attribute string,
	oldValue interface{},
	newValue interface{},
	actor blockchain.Address,
	blockHeight uint64,
	txHash blockchain.Hash,
) error {
	event := &StateChangeEvent{
		ID:          generateEventID(),
		Type:        ChangeTypeStatusChanged,
		EntityID:    entityID,
		EntityType:  entityType,
		Timestamp:   time.Now().Unix(),
		BlockHeight: blockHeight,
		TxHash:      txHash,
		Attribute:   attribute,
		OldValue:    oldValue,
		NewValue:    newValue,
		ActorID:     actor,
	}
	return s.RecordEvent(event)
}

// createAuditRecord creates an audit log record (Task 8.8)
func (s *Service) createAuditRecord(event *StateChangeEvent) error {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	record := &AuditLogRecord{
		ID:        generateRecordID(),
		Timestamp: time.Now().Unix(),
		PrevHash:  s.lastHash,
		EventHash: event.CalculateHash(),
		Event:     event,
	}

	// Calculate this record's hash
	recordHash := record.CalculateHash()
	s.lastHash = recordHash

	s.auditLog = append(s.auditLog, record)

	return nil
}

// GetEvents queries events for an entity (Task 8.5)
func (s *Service) GetEvents(entityID string, limit int) []*StateChangeEvent {
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()

	events, exists := s.events[entityID]
	if !exists {
		return []*StateChangeEvent{}
	}

	if limit <= 0 || limit > len(events) {
		return events
	}

	// Return most recent
	start := len(events) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*StateChangeEvent, len(events)-start)
	copy(result, events[start:])
	return result
}

// GetStateAtTime returns the state at a specific time (Task 8.6)
func (s *Service) GetStateAtTime(entityID string, timestamp int64) (*StateSnapshot, error) {
	s.eventsMu.RLock()
	events, exists := s.events[entityID]
	s.eventsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("entity not found: %s", entityID)
	}

	// Find events up to the timestamp
	var relevantEvents []*StateChangeEvent
	for _, event := range events {
		if event.Timestamp <= timestamp {
			relevantEvents = append(relevantEvents, event)
		}
	}

	// Build state snapshot
	snapshot := &StateSnapshot{
		Timestamp:  timestamp,
		EntityID:   entityID,
		EntityType: relevantEvents[0].EntityType,
		State:      make(map[string]interface{}),
	}

	// Apply events in order
	for _, event := range relevantEvents {
		if event.NewValue != nil {
			snapshot.State[event.Attribute] = event.NewValue
		}
		if event.BlockHeight > snapshot.BlockHeight {
			snapshot.BlockHeight = event.BlockHeight
		}
	}

	return snapshot, nil
}

// GetVehiclePlatoonHistory returns a vehicle's platoon history (Task 8.7)
func (s *Service) GetVehiclePlatoonHistory(vehicleID blockchain.Address) []*VehiclePlatoonHistory {
	s.historyMu.RLock()
	defer s.historyMu.RUnlock()

	history, exists := s.vehicleHistory[vehicleID]
	if !exists {
		return []*VehiclePlatoonHistory{}
	}

	// Return a copy
	result := make([]*VehiclePlatoonHistory, len(history))
	copy(result, history)
	return result
}

// updateVehicleHistory updates vehicle history
func (s *Service) updateVehicleHistory(event *StateChangeEvent) {
	if event.Type != ChangeTypeMemberJoined && event.Type != ChangeTypeMemberLeft {
		return
	}

	s.historyMu.Lock()
	defer s.historyMu.Unlock()

	vehicleID := blockchain.Address{}
	vehicleIDStr := event.EntityID
	copy(vehicleID[:], []byte(vehicleIDStr))

	platoonID, _ := event.Metadata["platoon_id"].(string)

	if event.Type == ChangeTypeMemberJoined {
		// Add new entry
		s.vehicleHistory[vehicleID] = append(s.vehicleHistory[vehicleID], &VehiclePlatoonHistory{
			VehicleID: vehicleID,
			PlatoonID: platoonID,
			JoinedAt:  event.Timestamp,
			Role:      "follower",
		})
	} else {
		// Update last entry with leave time
		history := s.vehicleHistory[vehicleID]
		for i := len(history) - 1; i >= 0; i-- {
			if history[i].PlatoonID == platoonID && history[i].LeftAt == 0 {
				history[i].LeftAt = event.Timestamp
				history[i].Duration = event.Timestamp - history[i].JoinedAt
				break
			}
		}
	}
}

// AnchorAuditLog anchors the audit log to the blockchain (Task 8.9)
func (s *Service) AnchorAuditLog(blockHeight uint64) error {
	s.auditMu.Lock()
	defer s.auditMu.Unlock()

	if len(s.auditLog) == 0 {
		return nil
	}

	// Get unanchored records
	var unanchored []*AuditLogRecord
	for _, record := range s.auditLog {
		if record.BlockHeight == 0 {
			unanchored = append(unanchored, record)
		}
	}

	if len(unanchored) == 0 {
		return nil
	}

	// Calculate Merkle root
	merkleRoot := s.calculateMerkleRoot(unanchored)

	// Update records with anchor info
	for _, record := range unanchored {
		record.BlockHeight = blockHeight
		record.MerkleRoot = merkleRoot
	}

	logger.Info("Audit log anchored",
		logger.Int("records", len(unanchored)),
		logger.Uint64("block_height", blockHeight),
		logger.String("merkle_root", merkleRoot.String()),
	)

	return nil
}

// calculateMerkleRoot calculates the Merkle root of audit records
func (s *Service) calculateMerkleRoot(records []*AuditLogRecord) blockchain.Hash {
	if len(records) == 0 {
		return blockchain.Hash{}
	}

	// Get hashes
	hashes := make([][]byte, len(records))
	for i, record := range records {
		hash := record.CalculateHash()
		hashes[i] = hash[:]
	}

	// Build Merkle tree
	for len(hashes) > 1 {
		if len(hashes)%2 == 1 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}

		newLevel := make([][]byte, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			concat := append(hashes[i], hashes[i+1]...)
			hash := crypto.Keccak256(concat)
			newLevel[i/2] = hash
		}
		hashes = newLevel
	}

	var result blockchain.Hash
	copy(result[:], hashes[0])
	return result
}

// checkForAnomalies checks for suspicious patterns (Task 8.15)
func (s *Service) checkForAnomalies(event *StateChangeEvent) {
	// Check for frequent join/leave
	if event.Type == ChangeTypeMemberJoined || event.Type == ChangeTypeMemberLeft {
		s.checkFrequentJoinLeave(event)
	}

	// Check for frequent leader changes
	if event.Type == ChangeTypeLeaderChanged {
		s.checkFrequentLeaderChanges(event)
	}
}

// checkFrequentJoinLeave checks if a vehicle is joining/leaving too frequently
func (s *Service) checkFrequentJoinLeave(event *StateChangeEvent) {
	// Look at recent events for this entity
	s.eventsMu.RLock()
	recentEvents := s.getRecentEvents(event.EntityID, 10)
	s.eventsMu.RUnlock()

	joinLeaveCount := 0
	for _, e := range recentEvents {
		if e.Type == ChangeTypeMemberJoined || e.Type == ChangeTypeMemberLeft {
			joinLeaveCount++
		}
	}

	// If more than 5 join/leave in recent history, flag as anomaly
	if joinLeaveCount > 5 {
		s.reportAnomaly(&AnomalyReport{
			ID:          generateAnomalyID(),
			Timestamp:   time.Now().Unix(),
			Type:        AnomalyTypeFrequentJoinLeave,
			Severity:    SeverityMedium,
			EntityID:    event.EntityID,
			Description: fmt.Sprintf("Vehicle joined/left %d times recently", joinLeaveCount),
			Evidence: map[string]interface{}{
				"recent_events": joinLeaveCount,
			},
		})
	}
}

// checkFrequentLeaderChanges checks for too many leader changes
func (s *Service) checkFrequentLeaderChanges(event *StateChangeEvent) {
	s.eventsMu.RLock()
	recentEvents := s.getRecentEvents(event.EntityID, 20)
	s.eventsMu.RUnlock()

	leaderChangeCount := 0
	for _, e := range recentEvents {
		if e.Type == ChangeTypeLeaderChanged {
			leaderChangeCount++
		}
	}

	// If more than 3 leader changes in recent history, flag as anomaly
	if leaderChangeCount > 3 {
		s.reportAnomaly(&AnomalyReport{
			ID:          generateAnomalyID(),
			Timestamp:   time.Now().Unix(),
			Type:        AnomalyTypeLeaderFrequentChange,
			Severity:    SeverityHigh,
			EntityID:    event.EntityID,
			Description: fmt.Sprintf("Platoon had %d leader changes recently", leaderChangeCount),
			Evidence: map[string]interface{}{
				"recent_changes": leaderChangeCount,
			},
		})
	}
}

// getRecentEvents returns recent events for an entity
func (s *Service) getRecentEvents(entityID string, limit int) []*StateChangeEvent {
	events, exists := s.events[entityID]
	if !exists {
		return nil
	}

	// Get the most recent
	start := len(events) - limit
	if start < 0 {
		start = 0
	}
	return events[start:]
}

// reportAnomaly reports an anomaly (Task 8.16)
func (s *Service) reportAnomaly(report *AnomalyReport) {
	s.anomalyMu.Lock()
	s.anomalies = append(s.anomalies, report)
	s.anomalyMu.Unlock()

	logger.Warn("Anomaly detected",
		logger.String("type", report.Type.String()),
		logger.String("severity", report.Severity.String()),
		logger.String("entity", report.EntityID),
		logger.String("description", report.Description),
	)
}

// GetAnomalies returns all anomalies
func (s *Service) GetAnomalies() []*AnomalyReport {
	s.anomalyMu.RLock()
	defer s.anomalyMu.RUnlock()

	result := make([]*AnomalyReport, len(s.anomalies))
	copy(result, s.anomalies)
	return result
}

// Subscribe subscribes to state change events (Task 8.11)
func (s *Service) Subscribe(handler func(*StateChangeEvent)) {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	s.subscribers = append(s.subscribers, handler)
}

// notifySubscribers notifies all subscribers
func (s *Service) notifySubscribers(event *StateChangeEvent) {
	s.subMu.RLock()
	subscribers := make([]func(*StateChangeEvent), len(s.subscribers))
	copy(subscribers, s.subscribers)
	s.subMu.RUnlock()

	for _, handler := range subscribers {
		go handler(event)
	}
}

// GenerateStatistics generates operational statistics (Task 8.17)
func (s *Service) GenerateStatistics(periodStart, periodEnd int64) *StatisticsReport {
	report := &StatisticsReport{
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
	}

	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()

	activePlatoons := make(map[string]bool)

	for _, events := range s.events {
		for _, event := range events {
			if event.Timestamp < periodStart || event.Timestamp > periodEnd {
				continue
			}

			switch event.Type {
			case ChangeTypePlatoonCreated:
				report.PlatoonsCreated++
				activePlatoons[event.EntityID] = true
			case ChangeTypePlatoonDissolved:
				report.PlatoonsDissolved++
				delete(activePlatoons, event.EntityID)
			case ChangeTypeMemberJoined:
				report.TotalJoins++
			case ChangeTypeMemberLeft:
				report.TotalLeaves++
			case ChangeTypeLeaderChanged:
				report.LeaderChanges++
			}
		}
	}

	report.PlatoonsActive = len(activePlatoons)

	return report
}

// CompressData compresses data for storage (Task 8.13)
func (s *Service) CompressData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	
	if err := writer.Close(); err != nil {
		return nil, err
	}
	
	return buf.Bytes(), nil
}

// DecompressData decompresses data
func (s *Service) DecompressData(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var result bytes.Buffer
	if _, err := result.ReadFrom(reader); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

// ArchiveToCloud archives data to cloud storage (Task 8.14)
func (s *Service) ArchiveToCloud(entityID string, startTime, endTime int64) (*CloudArchiveInfo, error) {
	// In a real implementation, this would upload to cloud storage
	// For now, just return info about what would be archived
	
	s.eventsMu.RLock()
	events, exists := s.events[entityID]
	s.eventsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("entity not found")
	}

	// Filter events in time range
	var toArchive []*StateChangeEvent
	for _, event := range events {
		if event.Timestamp >= startTime && event.Timestamp <= endTime {
			toArchive = append(toArchive, event)
		}
	}

	// Calculate Merkle root
	hashes := make([][]byte, len(toArchive))
	for i, event := range toArchive {
		hash := event.CalculateHash()
		hashes[i] = hash[:]
	}
	merkleRoot := calculateMerkleRoot(hashes)

	archiveInfo := &CloudArchiveInfo{
		ArchiveID:   generateArchiveID(),
		EntityID:    entityID,
		StartTime:   startTime,
		EndTime:     endTime,
		RecordCount: len(toArchive),
		MerkleRoot:  merkleRoot,
		ArchivedAt:  time.Now().Unix(),
	}

	logger.Info("Data archived to cloud",
		logger.String("archive_id", archiveInfo.ArchiveID),
		logger.String("entity", entityID),
		logger.Int("records", len(toArchive)),
	)

	return archiveInfo, nil
}

// Background tasks

func (s *Service) batchAnchorLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			// This would be triggered by block commits in real implementation
		}
	}
}

func (s *Service) retentionPolicyLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.enforceRetentionPolicy()
		}
	}
}

func (s *Service) enforceRetentionPolicy() {
	now := time.Now().Unix()
	
	// Move old events to compressed storage
	s.eventsMu.Lock()
	for entityID, events := range s.events {
		var toKeep []*StateChangeEvent
		for _, event := range events {
			age := (now - event.Timestamp) / (24 * 3600) // days
			if age <= int64(s.policy.HotStorageDays) {
				toKeep = append(toKeep, event)
			}
		}
		
		if len(toKeep) < len(events) {
			logger.Debug("Applying retention policy",
				logger.String("entity", entityID),
				logger.Int("removed", len(events)-len(toKeep)),
			)
			s.events[entityID] = toKeep
		}
	}
	s.eventsMu.Unlock()
}

// Storage helpers

func (s *Service) loadEvents() error {
	// Load from storage if needed
	return nil
}

func (s *Service) loadAuditLog() error {
	// Load from storage if needed
	return nil
}

// Helper functions

func generateEventID() string {
	return fmt.Sprintf("EVT-%d-%s", time.Now().Unix(), randomString(8))
}

func generateRecordID() string {
	return fmt.Sprintf("AUD-%d-%s", time.Now().Unix(), randomString(8))
}

func generateAnomalyID() string {
	return fmt.Sprintf("ANM-%d-%s", time.Now().Unix(), randomString(8))
}

func generateArchiveID() string {
	return fmt.Sprintf("ARC-%d-%s", time.Now().Unix(), randomString(8))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func calculateMerkleRoot(hashes [][]byte) blockchain.Hash {
	if len(hashes) == 0 {
		return blockchain.Hash{}
	}

	h := make([][]byte, len(hashes))
	copy(h, hashes)

	for len(h) > 1 {
		if len(h)%2 == 1 {
			h = append(h, h[len(h)-1])
		}

		newLevel := make([][]byte, len(h)/2)
		for i := 0; i < len(h); i += 2 {
			concat := append(h[i], h[i+1]...)
			hash := crypto.Keccak256(concat)
			newLevel[i/2] = hash
		}
		h = newLevel
	}

	var result blockchain.Hash
	copy(result[:], h[0])
	return result
}
