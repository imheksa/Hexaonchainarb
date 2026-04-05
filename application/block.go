package application

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
)

// Block represents a finalized Appchain block.
type Block struct {
	BlockNum     uint64
	BlockHash    [32]byte
	ParentHash   [32]byte
	Root         [32]byte
	Transactions []Transaction
}

// BlockConstructor satisfies gosdk.AppchainBlockConstructor.
func BlockConstructor(
	blockNumber uint64,
	stateRoot [32]byte,
	previousBlockHash [32]byte,
	batch apptypes.Batch[Transaction, Receipt],
) *Block {
	h := sha256.New()
	var blockNumBytes [8]byte
	binary.BigEndian.PutUint64(blockNumBytes[:], blockNumber)
	h.Write(blockNumBytes[:])
	h.Write(stateRoot[:])
	h.Write(previousBlockHash[:])

	var blockHash [32]byte
	copy(blockHash[:], h.Sum(nil))

	return &Block{
		BlockNum:     blockNumber,
		BlockHash:    blockHash,
		Root:         stateRoot,
		ParentHash:   previousBlockHash,
		Transactions: batch.Transactions,
	}
}
