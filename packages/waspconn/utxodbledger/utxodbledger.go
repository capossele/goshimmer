package utxodbledger

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxodb"
	"golang.org/x/xerrors"
)

// UtxoDBLedger implements waspconn.Ledger by wrapping UTXODB
type UtxoDBLedger struct {
	*utxodb.UtxoDB
	txConfirmedCallback func(tx *ledgerstate.Transaction)
}

func New() *UtxoDBLedger {
	return &UtxoDBLedger{UtxoDB: utxodb.New()}
}

func (u *UtxoDBLedger) PostTransaction(tx *ledgerstate.Transaction) error {
	err := u.AddTransaction(tx)
	if err != nil && u.txConfirmedCallback != nil {
		u.txConfirmedCallback(tx)
	}
	return err
}

func (u *UtxoDBLedger) GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output)) {
	for _, out := range u.GetAddressOutputs(addr) {
		f(out)
	}
}

func (u *UtxoDBLedger) GetConfirmedTransaction(txid ledgerstate.TransactionID, f func(*ledgerstate.Transaction)) bool {
	tx, ok := u.UtxoDB.GetTransaction(txid)
	if ok {
		f(tx)
	}
	return ok
}

func (u *UtxoDBLedger) GetTxInclusionState(txid ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	_, ok := u.UtxoDB.GetTransaction(txid)
	if !ok {
		return ledgerstate.Pending, xerrors.New("Not found")
	}
	return ledgerstate.Confirmed, nil
}

func (u *UtxoDBLedger) RequestFunds(target ledgerstate.Address) error {
	_, err := u.UtxoDB.RequestFunds(target)
	return err
}

func (u *UtxoDBLedger) OnTransactionConfirmed(cb func(tx *ledgerstate.Transaction)) {
	u.txConfirmedCallback = cb
}

func (u *UtxoDBLedger) OnTransactionBooked(f func(_ *ledgerstate.Transaction)) {}

func (u *UtxoDBLedger) Detach() {}
