package platoon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
)

func TestPlatoon_IsActive(t *testing.T) {
	p := &Platoon{
		ID:     "test-platoon",
		Status: PlatoonStatusActive,
	}
	
	assert.True(t, p.IsActive(), "Active platoon should be active")
	
	p.Status = PlatoonStatusDissolved
	assert.False(t, p.IsActive(), "Dissolved platoon should not be active")
	
	p.Status = PlatoonStatusForming
	assert.False(t, p.IsActive(), "Forming platoon should not be active")
}

func TestPlatoon_HasMember(t *testing.T) {
	vehicle1 := blockchain.Address{1, 2, 3}
	vehicle2 := blockchain.Address{4, 5, 6}
	
	p := &Platoon{
		ID: "test-platoon",
		Members: []*PlatoonMember{
			{VehicleID: vehicle1},
		},
	}
	
	assert.True(t, p.HasMember(vehicle1), "Should have member")
	assert.False(t, p.HasMember(vehicle2), "Should not have non-member")
}

func TestPlatoon_GetMemberCount(t *testing.T) {
	p := &Platoon{
		ID: "test-platoon",
		Members: []*PlatoonMember{
			{VehicleID: blockchain.Address{1}},
			{VehicleID: blockchain.Address{2}},
			{VehicleID: blockchain.Address{3}},
		},
	}
	
	assert.Equal(t, 3, p.GetMemberCount(), "Should have 3 members")
}

func TestPlatoon_IsValidator(t *testing.T) {
	vehicle1 := blockchain.Address{1}
	vehicle2 := blockchain.Address{2}
	
	p := &Platoon{
		ID:         "test-platoon",
		Validators: []blockchain.Address{vehicle1},
	}
	
	assert.True(t, p.IsValidator(vehicle1), "Should be validator")
	assert.False(t, p.IsValidator(vehicle2), "Should not be validator")
}

func TestPlatoonParams_Validate(t *testing.T) {
	// Valid params
	params := PlatoonParams{
		MaxVehicles:  8,
		TargetSpeed:  30.0,
		SafeDistance: 20.0,
	}
	
	err := params.Validate()
	assert.NoError(t, err, "Valid params should pass")
	
	// Invalid max vehicles (too small)
	params.MaxVehicles = 2
	err = params.Validate()
	assert.Error(t, err, "Too small max vehicles should fail")
	
	// Invalid max vehicles (too large)
	params.MaxVehicles = 200
	err = params.Validate()
	assert.Error(t, err, "Too large max vehicles should fail")
	
	// Reset and test invalid safe distance
	params.MaxVehicles = 8
	params.SafeDistance = 2.0
	err = params.Validate()
	assert.Error(t, err, "Too small safe distance should fail")
}

func TestJoinRequest(t *testing.T) {
	req := &JoinRequest{
		ID:          "req-001",
		PlatoonID:   "platoon-001",
		VehicleID:   blockchain.Address{1, 2, 3},
		Destination: "Destination A",
		Status:      JoinStatusPending,
	}
	
	assert.Equal(t, "req-001", req.ID)
	assert.Equal(t, "platoon-001", req.PlatoonID)
	assert.Equal(t, JoinStatusPending, req.Status)
}

func TestPlatoonStatus_String(t *testing.T) {
	assert.Equal(t, "forming", PlatoonStatusForming.String())
	assert.Equal(t, "active", PlatoonStatusActive.String())
	assert.Equal(t, "dissolved", PlatoonStatusDissolved.String())
	assert.Equal(t, "unknown", PlatoonStatus(99).String())
}

func TestEventType(t *testing.T) {
	// Just verify constants exist
	_ = EventPlatoonCreated
	_ = EventMemberJoined
	_ = EventMemberLeft
	_ = EventLeaderChanged
}

func TestMemberRole_String(t *testing.T) {
	assert.Equal(t, "leader", RoleLeader.String())
	assert.Equal(t, "follower", RoleFollower.String())
	assert.Equal(t, "validator", RoleValidator.String())
	assert.Equal(t, "none", MemberRole(99).String())
}

func TestDefaultPlatoonParams(t *testing.T) {
	params := DefaultPlatoonParams()
	
	assert.Equal(t, 100, params.MaxVehicles, "Default max vehicles should be 100")
	assert.Equal(t, 30.0, params.TargetSpeed, "Default target speed should be 30")
	assert.Equal(t, 20.0, params.SafeDistance, "Default safe distance should be 20")
	assert.Equal(t, 0, params.LaneID, "Default lane ID should be 0")
	assert.False(t, params.EmergencyMode, "Default emergency mode should be false")
}
