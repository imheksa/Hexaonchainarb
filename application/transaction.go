package application

import (
	"crypto/sha256"
	"encoding/json"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/kv"
)

// TransactionType represents admin actions sent to the Appchain.
type TransactionType string

const (
	TxTypeUpdateConfig TransactionType = "update_config"
	TxTypeWithdraw     TransactionType = "withdraw"
)

// Transaction is a manually submitted admin command for the Appchain.
// Arbitrage itself is triggered automatically by the ExtBlockProcessor
// when it detects price discrepancies — no manual transaction needed for that.
type Transaction struct {
	Type    TransactionType `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (t Transaction) Hash() [32]byte {
	h := sha256.New()
	h.Write([]byte(t.Type))
	h.Write(t.Payload)
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func (t Transaction) Process(dbTx kv.RwTx) (Receipt, []apptypes.ExternalTransaction, error) {
	hash := t.Hash()

	switch t.Type {
	case TxTypeUpdateConfig:
		return t.processUpdateConfig(dbTx, hash)
	case TxTypeWithdraw:
		return t.processWithdraw(dbTx, hash)
	default:
		return failedReceipt(hash, string(t.Type), ErrInvalidTransaction), nil, nil
	}
}

// UpdateConfigPayload lets operators adjust thresholds at runtime.
type UpdateConfigPayload struct {
	MinProfitBPS *int64  `json:"min_profit_bps,omitempty"`
	MaxBorrowUSD *uint64 `json:"max_borrow_usd,omitempty"`
	Enabled      *bool   `json:"enabled,omitempty"`
}

func (t Transaction) processUpdateConfig(dbTx kv.RwTx, hash [32]byte) (Receipt, []apptypes.ExternalTransaction, error) {
	var p UpdateConfigPayload
	if err := json.Unmarshal(t.Payload, &p); err != nil {
		return failedReceipt(hash, string(t.Type), err), nil, nil
	}

	if p.MinProfitBPS != nil {
		val, _ := json.Marshal(*p.MinProfitBPS)
		if err := dbTx.Put(ConfigBucket, []byte("min_profit_bps"), val); err != nil {
			return failedReceipt(hash, string(t.Type), err), nil, nil
		}
	}
	if p.MaxBorrowUSD != nil {
		val, _ := json.Marshal(*p.MaxBorrowUSD)
		if err := dbTx.Put(ConfigBucket, []byte("max_borrow_usd"), val); err != nil {
			return failedReceipt(hash, string(t.Type), err), nil, nil
		}
	}
	if p.Enabled != nil {
		val := []byte("false")
		if *p.Enabled {
			val = []byte("true")
		}
		if err := dbTx.Put(ConfigBucket, []byte("enabled"), val); err != nil {
			return failedReceipt(hash, string(t.Type), err), nil, nil
		}
	}

	return successReceipt(hash, string(t.Type), "config updated"), nil, nil
}

func (t Transaction) processWithdraw(dbTx kv.RwTx, hash [32]byte) (Receipt, []apptypes.ExternalTransaction, error) {
	// Withdraw triggers an ExternalTransaction to send accumulated stats/signal to owner
	// Actual on-chain withdrawal is handled by ArbitrageExecutor.withdrawProfit()
	return successReceipt(hash, string(t.Type), "withdrawal signal recorded"), nil, nil
}
