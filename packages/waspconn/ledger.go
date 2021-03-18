package waspconn

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// Ledger is the interface between waspconn and the underlying value tangle
type Ledger interface {
	GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output))
	GetConfirmedTransaction(txid ledgerstate.TransactionID, f func(*ledgerstate.Transaction)) bool
	GetTxInclusionState(txid ledgerstate.TransactionID) (ledgerstate.InclusionState, error)
	OnTransactionConfirmed(func(tx *ledgerstate.Transaction))
	OnTransactionBooked(func(tx *ledgerstate.Transaction))
	PostTransaction(tx *ledgerstate.Transaction) error
	RequestFunds(target ledgerstate.Address) error
	Detach()
}
