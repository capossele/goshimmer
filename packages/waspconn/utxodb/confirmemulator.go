package utxodb

import (
	"fmt"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxodb"
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

type pendingTransaction struct {
	confirmDeadline time.Time
	tx              *ledgerstate.Transaction
	hasConflicts    bool
}

// implements valuetangle.ValueTangle by wrapping UTXODB and adding a fake confirmation delay
type ConfirmEmulator struct {
	UtxoDB                 *utxodb.UtxoDB
	confirmTime            time.Duration
	randomize              bool
	confirmFirstInConflict bool
	pendingTransactions    map[ledgerstate.TransactionID]*pendingTransaction
	mutex                  sync.Mutex
	txConfirmedCallback    func(tx *ledgerstate.Transaction)
}

func NewConfirmEmulator(confirmTime time.Duration, randomize bool, confirmFirstInConflict bool) *ConfirmEmulator {
	ce := &ConfirmEmulator{
		UtxoDB:                 utxodb.New(),
		pendingTransactions:    make(map[ledgerstate.TransactionID]*pendingTransaction),
		confirmTime:            confirmTime,
		randomize:              randomize,
		confirmFirstInConflict: confirmFirstInConflict,
	}
	go ce.confirmLoop()
	return ce
}

func (ce *ConfirmEmulator) transactionConfirmed(tx *ledgerstate.Transaction) {
	if ce.txConfirmedCallback != nil {
		ce.txConfirmedCallback(tx)
	}
}

func (ce *ConfirmEmulator) PostTransaction(tx *ledgerstate.Transaction) error {
	ce.mutex.Lock()
	defer ce.mutex.Unlock()

	if ce.confirmTime == 0 {
		if err := ce.UtxoDB.AddTransaction(tx); err != nil {
			return err
		}
		ce.transactionConfirmed(tx)
		fmt.Printf("utxodb.ConfirmEmulator CONFIRMED IMMEDIATELY: %s\n", tx.ID().String())
		return nil
	}
	if err := ce.UtxoDB.AddTransaction().ValidateTransaction(tx); err != nil {
		return err
	}
	for txid, ptx := range ce.pendingTransactions {
		if areConflicting(tx, ptx.tx) {
			ptx.hasConflicts = true
			return fmt.Errorf("utxodb.ConfirmEmulator rejected: new tx %s conflicts with pending tx %s", tx.ID().String(), txid.String())
		}
	}
	var confTime time.Duration
	if ce.randomize {
		confTime = time.Duration(rand.Int31n(int32(ce.confirmTime)) + int32(ce.confirmTime)/2)
	} else {
		confTime = ce.confirmTime
	}
	deadline := time.Now().Add(confTime)

	ce.pendingTransactions[tx.ID()] = &pendingTransaction{
		confirmDeadline: deadline,
		tx:              tx,
		hasConflicts:    false,
	}
	fmt.Printf("utxodb.ConfirmEmulator ADDED PENDING TRANSACTION: %s\n", tx.ID().String())
	return nil
}

const loopPeriod = 500 * time.Millisecond

func (ce *ConfirmEmulator) confirmLoop() {
	maturedTxs := make([]ledgerstate.TransactionID, 0)
	for {
		time.Sleep(loopPeriod)

		maturedTxs = maturedTxs[:0]
		nowis := time.Now()
		ce.mutex.Lock()

		for txid, ptx := range ce.pendingTransactions {
			if ptx.confirmDeadline.Before(nowis) {
				maturedTxs = append(maturedTxs, txid)
			}
		}

		if len(maturedTxs) == 0 {
			ce.mutex.Unlock()
			continue
		}

		for _, txid := range maturedTxs {
			ptx := ce.pendingTransactions[txid]
			if ptx.hasConflicts && !ce.confirmFirstInConflict {
				// do not confirm if tx has conflicts
				fmt.Printf("!!! utxodb.ConfirmEmulator: rejected because has conflicts %s\n", txid.String())
				continue
			}
			if err := ce.UtxoDB.AddTransaction(ptx.tx); err != nil {
				fmt.Printf("!!!! utxodb.AddTransaction: %v\n", err)
			} else {
				ce.transactionConfirmed(ptx.tx)
				fmt.Printf("+++ utxodb.ConfirmEmulator: CONFIRMED %s after %v\n", txid.String(), ce.confirmTime)
			}
		}

		for _, txid := range maturedTxs {
			delete(ce.pendingTransactions, txid)
		}
		ce.mutex.Unlock()
	}
}

func (ce *ConfirmEmulator) GetAddressOutputs(addr ledgerstate.Address) map[ledgerstate.OutputID]*ledgerstate.ColoredBalances {
	ret := make(map[ledgerstate.OutputID]*ledgerstate.ColoredBalances)
	for _, output := range ce.UtxoDB.GetAddressOutputs(addr) {
		ret[output.ID()] = output.Balances()
	}
	return ret
}

func (ce *ConfirmEmulator) IsConfirmed(txid *ledgerstate.TransactionID) (bool, error) {
	return ce.UtxoDB.IsConfirmed(txid), nil
}

func (ce *ConfirmEmulator) GetTransaction(txid ledgerstate.TransactionID, f func(*ledgerstate.Transaction)) bool {
	tx, ok := ce.UtxoDB.GetTransaction(txid)
	if ok {
		f(tx)
	}
	return ok
}

func (ce *ConfirmEmulator) GetBranchInclusionState(txid ledgerstate.TransactionID) (ledgerstate.InclusionState, bool) {
	_, ok := ce.UtxoDB.GetTransaction(txid)
	if ok {
		return ledgerstate.Confirmed, true
	}
	return ledgerstate.Pending, false
}

func (ce *ConfirmEmulator) RequestFunds(target ledgerstate.Address) error {
	_, err := ce.UtxoDB.RequestFunds(target)
	return err
}

func (ce *ConfirmEmulator) OnTransactionConfirmed(cb func(tx *ledgerstate.Transaction)) {
	ce.txConfirmedCallback = cb
}

func (ce *ConfirmEmulator) OnTransactionBooked(f func(_ *ledgerstate.Transaction)) {
}

func (ce *ConfirmEmulator) Detach() {
	// TODO: stop the confirmLoop
}

// areConflicting checks if two transactions double-spend
func areConflicting(tx1, tx2 *ledgerstate.Transaction) bool {
	if tx1.ID() == tx2.ID() {
		return true
	}
	ret := false
	for _, oid1 := range tx1.Essence().Inputs() {
		for _, oid2 := range tx2.Essence().Inputs() {
			if oid1 == oid2 {
				ret = true
				return false
			}
			return true
		}
		return true
	}
	return ret
}
