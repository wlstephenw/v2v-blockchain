package executor

import (
	"encoding/json"
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/internal/core/blockchain"
)

// TransactionBuilder helps build transactions for different operations
type TransactionBuilder struct {
	nonce uint64
}

// NewTransactionBuilder creates a new transaction builder
func NewTransactionBuilder() *TransactionBuilder {
	return &TransactionBuilder{
		nonce: uint64(time.Now().UnixNano()),
	}
}

// BuildCreatePlatoonTx builds a transaction for creating a platoon
func (b *TransactionBuilder) BuildCreatePlatoonTx(
	from blockchain.Address,
	name string,
	safeDistance float64,
	maxSize int,
	targetSpeed float64,
	validators []blockchain.Address,
) *blockchain.Transaction {
	params := struct {
		Name         string               `json:"name"`
		SafeDistance float64              `json:"safe_distance"`
		MaxSize      int                  `json:"max_size"`
		TargetSpeed  float64              `json:"target_speed"`
		Validators   []blockchain.Address `json:"validators"`
	}{
		Name:         name,
		SafeDistance: safeDistance,
		MaxSize:      maxSize,
		TargetSpeed:  targetSpeed,
		Validators:   validators,
	}

	data, _ := json.Marshal(params)

	b.nonce++
	tx := &blockchain.Transaction{
		Type:      blockchain.TxType(TxTypeCreatePlatoon),
		From:      from,
		To:        blockchain.Address{},
		Value:     0,
		Data:      data,
		Nonce:     b.nonce,
		Timestamp: time.Now().Unix(),
	}
	tx.Hash = tx.CalculateHash()
	return tx
}

// BuildJoinPlatoonTx builds a transaction for joining a platoon
func (b *TransactionBuilder) BuildJoinPlatoonTx(
	from blockchain.Address,
	platoonID string,
	destination string,
) *blockchain.Transaction {
	params := struct {
		PlatoonID   string `json:"platoon_id"`
		Destination string `json:"destination"`
	}{
		PlatoonID:   platoonID,
		Destination: destination,
	}

	data, _ := json.Marshal(params)

	b.nonce++
	tx := &blockchain.Transaction{
		Type:      blockchain.TxType(TxTypeJoinPlatoon),
		From:      from,
		To:        blockchain.Address{},
		Value:     0,
		Data:      data,
		Nonce:     b.nonce,
		Timestamp: time.Now().Unix(),
	}
	tx.Hash = tx.CalculateHash()
	return tx
}

// BuildApproveJoinTx builds a transaction for approving a join request
func (b *TransactionBuilder) BuildApproveJoinTx(
	from blockchain.Address,
	requestID string,
	approved bool,
) *blockchain.Transaction {
	params := struct {
		RequestID string `json:"request_id"`
		Approved  bool   `json:"approved"`
	}{
		RequestID: requestID,
		Approved:  approved,
	}

	data, _ := json.Marshal(params)

	b.nonce++
	tx := &blockchain.Transaction{
		Type:      blockchain.TxType(TxTypeApproveJoin),
		From:      from,
		To:        blockchain.Address{},
		Value:     0,
		Data:      data,
		Nonce:     b.nonce,
		Timestamp: time.Now().Unix(),
	}
	tx.Hash = tx.CalculateHash()
	return tx
}

// BuildLeavePlatoonTx builds a transaction for leaving a platoon
func (b *TransactionBuilder) BuildLeavePlatoonTx(
	from blockchain.Address,
	platoonID string,
) *blockchain.Transaction {
	params := struct {
		PlatoonID string `json:"platoon_id"`
	}{
		PlatoonID: platoonID,
	}

	data, _ := json.Marshal(params)

	b.nonce++
	tx := &blockchain.Transaction{
		Type:      blockchain.TxType(TxTypeLeavePlatoon),
		From:      from,
		To:        blockchain.Address{},
		Value:     0,
		Data:      data,
		Nonce:     b.nonce,
		Timestamp: time.Now().Unix(),
	}
	tx.Hash = tx.CalculateHash()
	return tx
}

// BuildDissolvePlatoonTx builds a transaction for dissolving a platoon
func (b *TransactionBuilder) BuildDissolvePlatoonTx(
	from blockchain.Address,
	platoonID string,
) *blockchain.Transaction {
	params := struct {
		PlatoonID string `json:"platoon_id"`
		Reason    string `json:"reason"`
	}{
		PlatoonID: platoonID,
		Reason:    "leader_requested",
	}

	data, _ := json.Marshal(params)

	b.nonce++
	tx := &blockchain.Transaction{
		Type:      blockchain.TxType(TxTypeDissolvePlatoon),
		From:      from,
		To:        blockchain.Address{},
		Value:     0,
		Data:      data,
		Nonce:     b.nonce,
		Timestamp: time.Now().Unix(),
	}
	tx.Hash = tx.CalculateHash()
	return tx
}

// BuildAppointValidatorTx builds a transaction for appointing a validator
func (b *TransactionBuilder) BuildAppointValidatorTx(
	from blockchain.Address,
	platoonID string,
	vehicleID blockchain.Address,
) *blockchain.Transaction {
	params := struct {
		PlatoonID string             `json:"platoon_id"`
		VehicleID blockchain.Address `json:"vehicle_id"`
	}{
		PlatoonID: platoonID,
		VehicleID: vehicleID,
	}

	data, _ := json.Marshal(params)

	b.nonce++
	tx := &blockchain.Transaction{
		Type:      blockchain.TxType(TxTypeAppointValidator),
		From:      from,
		To:        blockchain.Address{},
		Value:     0,
		Data:      data,
		Nonce:     b.nonce,
		Timestamp: time.Now().Unix(),
	}
	tx.Hash = tx.CalculateHash()
	return tx
}

// BuildRegisterIdentityTx builds a transaction for registering identity
func (b *TransactionBuilder) BuildRegisterIdentityTx(
	from blockchain.Address,
	publicKey []byte,
	certificate []byte,
) *blockchain.Transaction {
	params := struct {
		PublicKey   []byte `json:"public_key"`
		Certificate []byte `json:"certificate"`
	}{
		PublicKey:   publicKey,
		Certificate: certificate,
	}

	data, _ := json.Marshal(params)

	b.nonce++
	tx := &blockchain.Transaction{
		Type:      blockchain.TxType(TxTypeRegisterIdentity),
		From:      from,
		To:        blockchain.Address{},
		Value:     0,
		Data:      data,
		Nonce:     b.nonce,
		Timestamp: time.Now().Unix(),
	}
	tx.Hash = tx.CalculateHash()
	return tx
}
