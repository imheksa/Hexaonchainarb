package application

type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrInvalidTransaction   = Error("invalid transaction type")
	ErrInsufficientProfit   = Error("arbitrage profit below minimum threshold")
	ErrNoOpportunity        = Error("no arbitrage opportunity detected")
	ErrPriceDataMissing     = Error("price data not available for pair")
	ErrContractNotDeployed  = Error("arbitrage executor contract address not set")
	ErrMissingParameters    = Error("missing required parameters")
	ErrDatabaseNotAvailable = Error("database not available")
	ErrInvalidPoolAddress   = Error("invalid pool address")
	ErrABIEncoding          = Error("failed to ABI encode payload")
)
