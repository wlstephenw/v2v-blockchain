package executor

import (
	"encoding/json"
	"fmt"

	"github.com/v2v-blockchain/v2v-blockchain/internal/blockchain"
	"github.com/v2v-blockchain/v2v-blockchain/internal/identity"
	"github.com/v2v-blockchain/v2v-blockchain/internal/platoon"
	"github.com/v2v-blockchain/v2v-blockchain/internal/state"
	"github.com/v2v-blockchain/v2v-blockchain/pkg/logger"
)

// Use identity package types
var _ = identity.RegistrationRequest{}

// TransactionType defines the type of transaction
type TransactionType uint8

const (
	// System transactions
	TxTypeRegisterIdentity TransactionType = iota
	TxTypeUpdateCertificate

	// Platoon transactions
	TxTypeCreatePlatoon
	TxTypeJoinPlatoon
	TxTypeApproveJoin
	TxTypeLeavePlatoon
	TxTypeDissolvePlatoon
	TxTypeAppointValidator
	TxTypeDemoteValidator

	// Message transactions
	TxTypeVerifyMessage
)

// Executor executes transactions from committed blocks
type Executor struct {
	platoon  *platoon.Service
	identity *identity.Service
	state    *state.Service
}

// NewExecutor creates a new transaction executor
func NewExecutor(
	platoonSvc *platoon.Service,
	identitySvc *identity.Service,
	stateSvc *state.Service,
) *Executor {
	return &Executor{
		platoon:  platoonSvc,
		identity: identitySvc,
		state:    stateSvc,
	}
}

// ExecuteBlock executes all transactions in a block
func (e *Executor) ExecuteBlock(block *blockchain.Block) error {
	logger.Info("Executing block",
		logger.Uint64("height", block.Header.Height),
		logger.Int("tx_count", len(block.Transactions)),
	)

	for i, tx := range block.Transactions {
		if err := e.ExecuteTransaction(tx, block.Header.Height, uint64(i)); err != nil {
			logger.Error("Failed to execute transaction",
				logger.String("hash", tx.Hash.String()),
				logger.ErrField(err),
			)
			// Continue executing other transactions, but log the error
			// In a production system, you might want to handle this differently
		}
	}

	logger.Info("Block execution completed",
		logger.Uint64("height", block.Header.Height),
	)
	return nil
}

// ExecuteTransaction executes a single transaction
func (e *Executor) ExecuteTransaction(tx *blockchain.Transaction, blockHeight uint64, txIndex uint64) error {
	// Determine transaction type from payload
	txType := TransactionType(tx.Type)

	logger.Debug("Executing transaction",
		logger.String("hash", tx.Hash.String()),
		logger.Int("type", int(txType)),
		logger.String("from", tx.From.String()),
	)

	switch txType {
	case TxTypeRegisterIdentity:
		return e.executeRegisterIdentity(tx)
	case TxTypeCreatePlatoon:
		return e.executeCreatePlatoon(tx)
	case TxTypeJoinPlatoon:
		return e.executeJoinPlatoon(tx)
	case TxTypeApproveJoin:
		return e.executeApproveJoin(tx)
	case TxTypeLeavePlatoon:
		return e.executeLeavePlatoon(tx)
	case TxTypeDissolvePlatoon:
		return e.executeDissolvePlatoon(tx)
	case TxTypeAppointValidator:
		return e.executeAppointValidator(tx)
	case TxTypeDemoteValidator:
		return e.executeDemoteValidator(tx)
	default:
		logger.Warn("Unknown transaction type",
			logger.Int("type", int(txType)),
			logger.String("hash", tx.Hash.String()),
		)
		return nil
	}
}

// executeRegisterIdentity executes identity registration
func (e *Executor) executeRegisterIdentity(tx *blockchain.Transaction) error {
	var params struct {
		PublicKey   []byte `json:"public_key"`
		Certificate []byte `json:"certificate"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal register identity params: %w", err)
	}

	req := &identity.RegistrationRequest{
		VehicleID:   tx.From,
		PublicKey:   params.PublicKey,
		Certificate: params.Certificate,
	}

	_, err := e.identity.RegisterVehicle(req)
	if err != nil {
		return fmt.Errorf("failed to register identity: %w", err)
	}

	logger.Info("Identity registered via block execution",
		logger.String("address", tx.From.String()),
	)
	return nil
}

// executeCreatePlatoon executes platoon creation
func (e *Executor) executeCreatePlatoon(tx *blockchain.Transaction) error {
	var params struct {
		Name         string               `json:"name"`
		SafeDistance float64              `json:"safe_distance"`
		MaxSize      int                  `json:"max_size"`
		TargetSpeed  float64              `json:"target_speed"`
		Validators   []blockchain.Address `json:"validators"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal create platoon params: %w", err)
	}

	platoonParams := platoon.PlatoonParams{
		SafeDistance: params.SafeDistance,
		MaxVehicles:  params.MaxSize,
		TargetSpeed:  params.TargetSpeed,
	}

	plat, err := e.platoon.CreatePlatoon(tx.From, platoonParams, params.Name)
	if err != nil {
		return fmt.Errorf("failed to create platoon: %w", err)
	}

	// Add validators if specified
	for _, validator := range params.Validators {
		if err := e.platoon.AppointValidator(plat.ID, validator, tx.From); err != nil {
			logger.Warn("Failed to appoint validator",
				logger.String("platoon", plat.ID),
				logger.String("validator", validator.String()),
				logger.ErrField(err),
			)
		}
	}

	logger.Info("Platoon created via block execution",
		logger.String("platoon_id", plat.ID),
		logger.String("leader", tx.From.String()),
	)
	return nil
}

// executeJoinPlatoon executes join platoon request
func (e *Executor) executeJoinPlatoon(tx *blockchain.Transaction) error {
	var params struct {
		PlatoonID   string `json:"platoon_id"`
		Destination string `json:"destination"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal join platoon params: %w", err)
	}

	req, err := e.platoon.SubmitJoinRequest(params.PlatoonID, tx.From, params.Destination)
	if err != nil {
		return fmt.Errorf("failed to submit join request: %w", err)
	}

	logger.Info("Join request submitted via block execution",
		logger.String("request_id", req.ID),
		logger.String("platoon_id", params.PlatoonID),
		logger.String("vehicle", tx.From.String()),
	)
	return nil
}

// executeApproveJoin executes join approval
func (e *Executor) executeApproveJoin(tx *blockchain.Transaction) error {
	var params struct {
		RequestID string `json:"request_id"`
		Approved  bool   `json:"approved"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal approve join params: %w", err)
	}

	if err := e.platoon.ProcessJoinRequest(params.RequestID, params.Approved, tx.From); err != nil {
		return fmt.Errorf("failed to process join request: %w", err)
	}

	logger.Info("Join request processed via block execution",
		logger.String("request_id", params.RequestID),
		logger.Bool("approved", params.Approved),
	)
	return nil
}

// executeLeavePlatoon executes leave platoon
func (e *Executor) executeLeavePlatoon(tx *blockchain.Transaction) error {
	var params struct {
		PlatoonID string `json:"platoon_id"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal leave platoon params: %w", err)
	}

	if err := e.platoon.LeavePlatoon(params.PlatoonID, tx.From); err != nil {
		return fmt.Errorf("failed to leave platoon: %w", err)
	}

	logger.Info("Vehicle left platoon via block execution",
		logger.String("platoon_id", params.PlatoonID),
		logger.String("vehicle", tx.From.String()),
	)
	return nil
}

// executeDissolvePlatoon executes platoon dissolution
func (e *Executor) executeDissolvePlatoon(tx *blockchain.Transaction) error {
	var params struct {
		PlatoonID string `json:"platoon_id"`
		Reason    string `json:"reason"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal dissolve platoon params: %w", err)
	}

	// Verify the sender is the leader
	plat, exists := e.platoon.GetPlatoon(params.PlatoonID)
	if !exists {
		return fmt.Errorf("platoon not found: %s", params.PlatoonID)
	}
	if plat.LeaderID != tx.From {
		return fmt.Errorf("only leader can dissolve platoon")
	}

	if err := e.platoon.DissolvePlatoon(params.PlatoonID, params.Reason); err != nil {
		return fmt.Errorf("failed to dissolve platoon: %w", err)
	}

	logger.Info("Platoon dissolved via block execution",
		logger.String("platoon_id", params.PlatoonID),
	)
	return nil
}

// executeAppointValidator executes validator appointment
func (e *Executor) executeAppointValidator(tx *blockchain.Transaction) error {
	var params struct {
		PlatoonID string             `json:"platoon_id"`
		VehicleID blockchain.Address `json:"vehicle_id"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal appoint validator params: %w", err)
	}

	if err := e.platoon.AppointValidator(params.PlatoonID, params.VehicleID, tx.From); err != nil {
		return fmt.Errorf("failed to appoint validator: %w", err)
	}

	logger.Info("Validator appointed via block execution",
		logger.String("platoon_id", params.PlatoonID),
		logger.String("vehicle", params.VehicleID.String()),
	)
	return nil
}

// executeDemoteValidator executes validator demotion
func (e *Executor) executeDemoteValidator(tx *blockchain.Transaction) error {
	var params struct {
		PlatoonID string             `json:"platoon_id"`
		VehicleID blockchain.Address `json:"vehicle_id"`
	}
	if err := json.Unmarshal(tx.Data, &params); err != nil {
		return fmt.Errorf("failed to unmarshal demote validator params: %w", err)
	}

	if err := e.platoon.DemoteValidator(params.PlatoonID, params.VehicleID, tx.From); err != nil {
		return fmt.Errorf("failed to demote validator: %w", err)
	}

	logger.Info("Validator demoted via block execution",
		logger.String("platoon_id", params.PlatoonID),
		logger.String("vehicle", params.VehicleID.String()),
	)
	return nil
}
