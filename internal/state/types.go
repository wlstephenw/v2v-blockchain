package state

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
)

// ChangeType represents the type of state change
type ChangeType uint8

const (
	ChangeTypePlatoonCreated ChangeType = iota
	ChangeTypePlatoonDissolved
	ChangeTypeMemberJoined
	ChangeTypeMemberLeft
	ChangeTypeLeaderChanged
	ChangeTypeValidatorAdded
	ChangeTypeValidatorRemoved
	ChangeTypeParamsUpdated
	ChangeTypeStatusChanged
	ChangeTypeIdentityRegistered
	ChangeTypeCertificateUpdated
	ChangeTypeCertificateRevoked
)

func (t ChangeType) String() string {
	switch t {
	case ChangeTypePlatoonCreated:
		return "platoon_created"
	case ChangeTypePlatoonDissolved:
		return "platoon_dissolved"
	case ChangeTypeMemberJoined:
		return "member_joined"
	case ChangeTypeMemberLeft:
		return "member_left"
	case ChangeTypeLeaderChanged:
		return "leader_changed"
	case ChangeTypeValidatorAdded:
		return "validator_added"
	case ChangeTypeValidatorRemoved:
		return "validator_removed"
	case ChangeTypeParamsUpdated:
		return "params_updated"
	case ChangeTypeStatusChanged:
		return "status_changed"
	case ChangeTypeIdentityRegistered:
		return "identity_registered"
	case ChangeTypeCertificateUpdated:
		return "certificate_updated"
	case ChangeTypeCertificateRevoked:
		return "certificate_revoked"
	default:
		return "unknown"
	}
}

// StateChangeEvent represents a state change event (Task 8.1)
type StateChangeEvent struct {
	ID          string             `json:"id"`
	Type        ChangeType         `json:"type"`
	EntityID    string             `json:"entity_id"`   // Platoon ID or Vehicle ID
	EntityType  string             `json:"entity_type"` // "platoon", "vehicle", "identity"
	Timestamp   int64              `json:"timestamp"`
	BlockHeight uint64             `json:"block_height"`
	TxHash      blockchain.Hash    `json:"tx_hash"`
	
	// Change details (Task 8.4)
	Attribute   string      `json:"attribute,omitempty"`
	OldValue    interface{} `json:"old_value,omitempty"`
	NewValue    interface{} `json:"new_value,omitempty"`
	
	// Actor
	ActorID     blockchain.Address `json:"actor_id"` // Who made the change
	
	// Additional data
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// CalculateHash calculates the hash of the event
func (e *StateChangeEvent) CalculateHash() blockchain.Hash {
	data, _ := json.Marshal(e)
	hash := crypto.Keccak256Hash(data)
	var result [32]byte
	copy(result[:], hash[:])
	return result
}

// AuditLogRecord represents a single audit log record (Task 8.8)
type AuditLogRecord struct {
	ID           string             `json:"id"`
	Timestamp    int64              `json:"timestamp"`
	PrevHash     blockchain.Hash    `json:"prev_hash"`  // Hash of previous record
	EventHash    blockchain.Hash    `json:"event_hash"` // Hash of the state change event
	Event        *StateChangeEvent  `json:"event"`
	MerkleRoot   blockchain.Hash    `json:"merkle_root,omitempty"` // Root of batch
	BlockHeight  uint64             `json:"block_height,omitempty"` // When anchored to chain
}

// CalculateHash calculates the hash of the audit record
func (r *AuditLogRecord) CalculateHash() blockchain.Hash {
	data, _ := json.Marshal(struct {
		ID        string          `json:"id"`
		Timestamp int64           `json:"timestamp"`
		PrevHash  blockchain.Hash `json:"prev_hash"`
		EventHash blockchain.Hash `json:"event_hash"`
	}{
		ID:        r.ID,
		Timestamp: r.Timestamp,
		PrevHash:  r.PrevHash,
		EventHash: r.EventHash,
	})
	
	hash := crypto.Keccak256Hash(data)
	var result [32]byte
	copy(result[:], hash[:])
	return result
}

// StateSnapshot represents a state at a specific point in time (Task 8.6)
type StateSnapshot struct {
	Timestamp   int64              `json:"timestamp"`
	BlockHeight uint64             `json:"block_height"`
	EntityID    string             `json:"entity_id"`
	EntityType  string             `json:"entity_type"`
	State       map[string]interface{} `json:"state"`
}

// VehiclePlatoonHistory represents a vehicle's platoon participation history (Task 8.7)
type VehiclePlatoonHistory struct {
	VehicleID      blockchain.Address `json:"vehicle_id"`
	PlatoonID      string             `json:"platoon_id"`
	JoinedAt       int64              `json:"joined_at"`
	LeftAt         int64              `json:"left_at,omitempty"`
	Role           string             `json:"role"`
	Duration       int64              `json:"duration,omitempty"`
}

// RetentionPolicy defines data retention rules (Task 8.12)
type RetentionPolicy struct {
	HotStorageDays   int  // Recent data, fast access (default: 7 days)
	WarmStorageDays  int  // Compressed data, slower access (default: 365 days)
	ArchiveToCloud   bool // Archive to cloud after warm storage
}

// DefaultRetentionPolicy returns the default retention policy
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		HotStorageDays:  7,
		WarmStorageDays: 365,
		ArchiveToCloud:  true,
	}
}

// AnomalyReport represents a detected anomaly (Task 8.15)
type AnomalyReport struct {
	ID          string                 `json:"id"`
	Timestamp   int64                  `json:"timestamp"`
	Type        AnomalyType            `json:"type"`
	Severity    Severity               `json:"severity"`
	EntityID    string                 `json:"entity_id"`
	Description string                 `json:"description"`
	Evidence    map[string]interface{} `json:"evidence"`
	Acknowledged bool                  `json:"acknowledged"`
}

// AnomalyType represents the type of anomaly
type AnomalyType uint8

const (
	AnomalyTypeFrequentJoinLeave AnomalyType = iota
	AnomalyTypeLeaderFrequentChange
	AnomalyTypeSuspiciousMessagePattern
	AnomalyTypeValidatorFailure
	AnomalyTypeNetworkPartition
)

func (t AnomalyType) String() string {
	switch t {
	case AnomalyTypeFrequentJoinLeave:
		return "frequent_join_leave"
	case AnomalyTypeLeaderFrequentChange:
		return "leader_frequent_change"
	case AnomalyTypeSuspiciousMessagePattern:
		return "suspicious_message_pattern"
	case AnomalyTypeValidatorFailure:
		return "validator_failure"
	case AnomalyTypeNetworkPartition:
		return "network_partition"
	default:
		return "unknown"
	}
}

// Severity represents the severity level
type Severity uint8

const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// StatisticsReport represents operational statistics (Task 8.17)
type StatisticsReport struct {
	PeriodStart int64 `json:"period_start"`
	PeriodEnd   int64 `json:"period_end"`
	
	// Platoon statistics
	PlatoonsCreated   int `json:"platoons_created"`
	PlatoonsDissolved int `json:"platoons_dissolved"`
	PlatoonsActive    int `json:"platoons_active"`
	
	// Member statistics
	TotalJoins  int `json:"total_joins"`
	TotalLeaves int `json:"total_leaves"`
	
	// Leader statistics
	LeaderChanges int `json:"leader_changes"`
	
	// Performance statistics
	AvgPlatoonDuration float64 `json:"avg_platoon_duration"`
	AvgMemberDuration  float64 `json:"avg_member_duration"`
}

// EventFilter represents filters for querying events
type EventFilter struct {
	EntityID    string
	EntityType  string
	ChangeType  ChangeType
	StartTime   int64
	EndTime     int64
	BlockHeight uint64
}

// QueryResult represents the result of a state query
type QueryResult struct {
	Events  []*StateChangeEvent `json:"events"`
	Total   int                 `json:"total"`
	HasMore bool                `json:"has_more"`
}

// CloudArchiveInfo represents information about cloud archived data (Task 8.14)
type CloudArchiveInfo struct {
	ArchiveID   string `json:"archive_id"`
	EntityID    string `json:"entity_id"`
	StartTime   int64  `json:"start_time"`
	EndTime     int64  `json:"end_time"`
	RecordCount int    `json:"record_count"`
	MerkleRoot  blockchain.Hash `json:"merkle_root"`
	ArchiveURL  string `json:"archive_url,omitempty"`
	ArchivedAt  int64  `json:"archived_at"`
}

// IsEmpty checks if old value is empty
func (e *StateChangeEvent) HasOldValue() bool {
	return e.OldValue != nil
}

// IsEmpty checks if new value is empty
func (e *StateChangeEvent) HasNewValue() bool {
	return e.NewValue != nil
}

// Validate validates the event
func (e *StateChangeEvent) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("event ID is required")
	}
	if e.EntityID == "" {
		return fmt.Errorf("entity ID is required")
	}
	if e.Timestamp == 0 {
		return fmt.Errorf("timestamp is required")
	}
	return nil
}
