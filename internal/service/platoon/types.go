package platoon

import (
	"fmt"
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
)

// PlatoonStatus represents the status of a platoon
type PlatoonStatus uint8

const (
	PlatoonStatusInactive PlatoonStatus = iota
	PlatoonStatusActive
	PlatoonStatusForming
	PlatoonStatusDissolving
	PlatoonStatusDissolved
	PlatoonStatusUnderStaffed // Less than minimum validators
)

func (s PlatoonStatus) String() string {
	switch s {
	case PlatoonStatusInactive:
		return "inactive"
	case PlatoonStatusActive:
		return "active"
	case PlatoonStatusForming:
		return "forming"
	case PlatoonStatusDissolving:
		return "dissolving"
	case PlatoonStatusDissolved:
		return "dissolved"
	case PlatoonStatusUnderStaffed:
		return "understaffed"
	default:
		return "unknown"
	}
}

// MemberRole represents the role of a member in a platoon
type MemberRole uint8

const (
	RoleNone MemberRole = iota
	RoleLeader
	RoleValidator
	RoleFollower
)

func (r MemberRole) String() string {
	switch r {
	case RoleLeader:
		return "leader"
	case RoleValidator:
		return "validator"
	case RoleFollower:
		return "follower"
	default:
		return "none"
	}
}

// PlatoonParams represents the parameters for a platoon
type PlatoonParams struct {
	MaxVehicles    int     `json:"max_vehicles"`     // Maximum number of vehicles (default: 100)
	TargetSpeed    float64 `json:"target_speed"`     // Target speed in m/s
	SafeDistance   float64 `json:"safe_distance"`    // Safe following distance in meters
	LaneID         int     `json:"lane_id"`          // Lane ID
	RouteID        string  `json:"route_id"`         // Route ID
	EmergencyMode  bool    `json:"emergency_mode"`   // Emergency mode flag
}

// DefaultPlatoonParams returns default platoon parameters
func DefaultPlatoonParams() PlatoonParams {
	return PlatoonParams{
		MaxVehicles:  100,
		TargetSpeed:  30.0,  // ~108 km/h
		SafeDistance: 20.0,  // 20 meters
		LaneID:       0,
		EmergencyMode: false,
	}
}

// Validate validates platoon parameters
func (p *PlatoonParams) Validate() error {
	if p.MaxVehicles < 4 {
		return fmt.Errorf("max_vehicles must be at least 4, got %d", p.MaxVehicles)
	}
	if p.MaxVehicles > 100 {
		return fmt.Errorf("max_vehicles cannot exceed 100, got %d", p.MaxVehicles)
	}
	if p.TargetSpeed < 0 {
		return fmt.Errorf("target_speed must be non-negative")
	}
	if p.SafeDistance < 5.0 {
		return fmt.Errorf("safe_distance must be at least 5 meters")
	}
	return nil
}

// PlatoonMember represents a member of a platoon
type PlatoonMember struct {
	VehicleID    blockchain.Address `json:"vehicle_id"`
	Role         MemberRole         `json:"role"`
	JoinedAt     int64              `json:"joined_at"`
	Position     int                `json:"position"` // Position in platoon (0 = leader)
	Status       string             `json:"status"`
	LastSeenAt   int64              `json:"last_seen_at"`
}

// Platoon represents a vehicle platoon
type Platoon struct {
	ID          string             `json:"id"`
	Name        string             `json:"name,omitempty"`
	Status      PlatoonStatus      `json:"status"`
	Params      PlatoonParams      `json:"params"`
	LeaderID    blockchain.Address `json:"leader_id"`
	Members     []*PlatoonMember   `json:"members"`
	Validators  []blockchain.Address `json:"validators"` // List of validator addresses
	CreatedAt   int64              `json:"created_at"`
	UpdatedAt   int64              `json:"updated_at"`
	DissolvedAt int64              `json:"dissolved_at,omitempty"`
	
	// Blacklist
	Blacklist   []blockchain.Address `json:"blacklist,omitempty"`
	
	// Statistics
	BlockHeight uint64 `json:"block_height"` // Current block height in platoon's chain
	
	// Timeout tracking
	LastActivityAt int64 `json:"last_activity_at"`
	TimeoutSecs    int   `json:"timeout_secs"` // Auto-dissolve timeout (default: 300s = 5min)
}

// NewPlatoon creates a new platoon
func NewPlatoon(id string, leaderID blockchain.Address, params PlatoonParams) (*Platoon, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	now := time.Now().Unix()
	return &Platoon{
		ID:             id,
		Status:         PlatoonStatusForming,
		Params:         params,
		LeaderID:       leaderID,
		Members:        make([]*PlatoonMember, 0),
		Validators:     make([]blockchain.Address, 0),
		CreatedAt:      now,
		UpdatedAt:      now,
		LastActivityAt: now,
		TimeoutSecs:    300, // 5 minutes default
		Blacklist:      make([]blockchain.Address, 0),
	}, nil
}

// IsActive checks if the platoon is active
func (p *Platoon) IsActive() bool {
	return p.Status == PlatoonStatusActive || p.Status == PlatoonStatusUnderStaffed
}

// IsDissolved checks if the platoon is dissolved
func (p *Platoon) IsDissolved() bool {
	return p.Status == PlatoonStatusDissolved
}

// GetMember returns a member by vehicle ID
func (p *Platoon) GetMember(vehicleID blockchain.Address) (*PlatoonMember, bool) {
	for _, member := range p.Members {
		if member.VehicleID == vehicleID {
			return member, true
		}
	}
	return nil, false
}

// HasMember checks if a vehicle is a member
func (p *Platoon) HasMember(vehicleID blockchain.Address) bool {
	_, exists := p.GetMember(vehicleID)
	return exists
}

// IsBlacklisted checks if a vehicle is blacklisted
func (p *Platoon) IsBlacklisted(vehicleID blockchain.Address) bool {
	for _, id := range p.Blacklist {
		if id == vehicleID {
			return true
		}
	}
	return false
}

// AddMember adds a member to the platoon
func (p *Platoon) AddMember(vehicleID blockchain.Address, role MemberRole) error {
	if p.HasMember(vehicleID) {
		return fmt.Errorf("vehicle already in platoon")
	}
	if p.IsBlacklisted(vehicleID) {
		return fmt.Errorf("vehicle is blacklisted")
	}
	if len(p.Members) >= p.Params.MaxVehicles {
		return fmt.Errorf("platoon is full")
	}

	now := time.Now().Unix()
	member := &PlatoonMember{
		VehicleID:  vehicleID,
		Role:       role,
		JoinedAt:   now,
		Position:   len(p.Members),
		Status:     "active",
		LastSeenAt: now,
	}

	p.Members = append(p.Members, member)
	p.UpdatedAt = now
	p.LastActivityAt = now

	return nil
}

// RemoveMember removes a member from the platoon
func (p *Platoon) RemoveMember(vehicleID blockchain.Address) error {
	for i, member := range p.Members {
		if member.VehicleID == vehicleID {
			// Remove by swapping with last and truncating
			p.Members[i] = p.Members[len(p.Members)-1]
			p.Members = p.Members[:len(p.Members)-1]
			p.UpdatedAt = time.Now().Unix()
			
			// Reassign positions
			for j, m := range p.Members {
				m.Position = j
			}
			
			return nil
		}
	}
	return fmt.Errorf("member not found")
}

// GetValidatorCount returns the number of validators
func (p *Platoon) GetValidatorCount() int {
	return len(p.Validators)
}

// IsValidator checks if a vehicle is a validator
func (p *Platoon) IsValidator(vehicleID blockchain.Address) bool {
	for _, v := range p.Validators {
		if v == vehicleID {
			return true
		}
	}
	return false
}

// AddValidator adds a validator
func (p *Platoon) AddValidator(vehicleID blockchain.Address) error {
	if p.IsValidator(vehicleID) {
		return fmt.Errorf("already a validator")
	}
	p.Validators = append(p.Validators, vehicleID)
	
	// Update member role
	if member, exists := p.GetMember(vehicleID); exists {
		member.Role = RoleValidator
	}
	
	return nil
}

// RemoveValidator removes a validator
func (p *Platoon) RemoveValidator(vehicleID blockchain.Address) error {
	for i, v := range p.Validators {
		if v == vehicleID {
			p.Validators = append(p.Validators[:i], p.Validators[i+1:]...)
			
			// Update member role
			if member, exists := p.GetMember(vehicleID); exists {
				member.Role = RoleFollower
			}
			
			return nil
		}
	}
	return fmt.Errorf("validator not found")
}

// AddToBlacklist adds a vehicle to the blacklist
func (p *Platoon) AddToBlacklist(vehicleID blockchain.Address) {
	if !p.IsBlacklisted(vehicleID) {
		p.Blacklist = append(p.Blacklist, vehicleID)
	}
}

// GetMemberCount returns the number of members
func (p *Platoon) GetMemberCount() int {
	return len(p.Members)
}

// CanFormConsensus checks if the platoon has enough validators for consensus
func (p *Platoon) CanFormConsensus() bool {
	// Minimum 4 validators needed for PBFT (f=1, 2f+1=3, need 4 for safety)
	return len(p.Validators) >= 4
}

// UpdateActivity updates the last activity timestamp
func (p *Platoon) UpdateActivity() {
	p.LastActivityAt = time.Now().Unix()
}

// IsTimedOut checks if the platoon has timed out
func (p *Platoon) IsTimedOut() bool {
	return time.Now().Unix()-p.LastActivityAt > int64(p.TimeoutSecs)
}

// JoinRequest represents a request to join a platoon
type JoinRequest struct {
	ID           string             `json:"id"`
	PlatoonID    string             `json:"platoon_id"`
	VehicleID    blockchain.Address `json:"vehicle_id"`
	Destination  string             `json:"destination,omitempty"`
	RequestedAt  int64              `json:"requested_at"`
	Status       JoinRequestStatus  `json:"status"`
	ProcessedAt  int64              `json:"processed_at,omitempty"`
	ProcessedBy  blockchain.Address `json:"processed_by,omitempty"`
}

// JoinRequestStatus represents the status of a join request
type JoinRequestStatus uint8

const (
	JoinStatusPending JoinRequestStatus = iota
	JoinStatusApproved
	JoinStatusRejected
	JoinStatusExpired
)

// PlatoonEvent represents a platoon-related event
type PlatoonEvent struct {
	Type      EventType          `json:"type"`
	PlatoonID string             `json:"platoon_id"`
	VehicleID blockchain.Address `json:"vehicle_id,omitempty"`
	Timestamp int64              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// EventType represents the type of platoon event
type EventType uint8

const (
	EventPlatoonCreated EventType = iota
	EventPlatoonDissolved
	EventMemberJoined
	EventMemberLeft
	EventLeaderChanged
	EventValidatorAdded
	EventValidatorRemoved
	EventParamsUpdated
	EventBlacklisted
)

// PlatoonQueryResult represents the result of a platoon query
type PlatoonQueryResult struct {
	Platoon   *Platoon    `json:"platoon"`
	Found     bool        `json:"found"`
	Error     string      `json:"error,omitempty"`
}

// PlatoonHistoryEntry represents a historical entry for a platoon
type PlatoonHistoryEntry struct {
	Timestamp int64              `json:"timestamp"`
	Event     EventType          `json:"event"`
	VehicleID blockchain.Address `json:"vehicle_id,omitempty"`
	OldValue  interface{}        `json:"old_value,omitempty"`
	NewValue  interface{}        `json:"new_value,omitempty"`
	TxHash    blockchain.Hash    `json:"tx_hash,omitempty"`
}
