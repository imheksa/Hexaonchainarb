package application

import (
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/holiman/uint256"
)

// Receipt is the result of processing an admin Transaction on our Appchain.
type Receipt struct {
	TxnHash      [32]byte
	TxStatus     apptypes.TxReceiptStatus
	ErrorMessage string
	Action       string // e.g. "update_config", "withdraw"
	Detail       string // human-readable result
}

func (r Receipt) TxHash() [32]byte                    { return r.TxnHash }
func (r Receipt) Status() apptypes.TxReceiptStatus    { return r.TxStatus }
func (r Receipt) Error() string                        { return r.ErrorMessage }

// ArbitrageReceipt is stored when the Appchain detects and emits an arb opportunity.
type ArbitrageReceipt struct {
	BlockNumber    uint64
	Pair           string
	BuyDEX         string
	SellDEX        string
	Direction      uint8
	BorrowAmount   *uint256.Int
	ExpectedProfit *uint256.Int // in borrow token units
	ProfitBPS      int64
	Triggered      bool // true if ExternalTransaction was emitted
}

func successReceipt(hash [32]byte, action, detail string) Receipt {
	return Receipt{
		TxnHash:  hash,
		TxStatus: apptypes.TxReceiptStatusSuccess,
		Action:   action,
		Detail:   detail,
	}
}

func failedReceipt(hash [32]byte, action string, err error) Receipt {
	return Receipt{
		TxnHash:      hash,
		TxStatus:     apptypes.TxReceiptStatusFailure,
		Action:       action,
		ErrorMessage: err.Error(),
	}
}
