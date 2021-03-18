package tangleledger

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/waspconn"
	"github.com/iotaledger/goshimmer/plugins/config"
	"github.com/iotaledger/goshimmer/plugins/faucet"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/events"
)

// TangleLedger imlpements waspconn.TangleLedger with the Goshimmer tangle as backend
type TangleLedger struct {
	txConfirmedClosure  *events.Closure
	txConfirmedCallback func(tx *ledgerstate.Transaction)

	txBookedClosure  *events.Closure
	txBookedCallback func(tx *ledgerstate.Transaction)
}

// ensure conformance to Ledger interface
var _ waspconn.Ledger = &TangleLedger{}

func extractTransaction(id tangle.MessageID, f func(*ledgerstate.Transaction)) {
	if f == nil {
		return
	}
	messagelayer.Tangle().Storage.Message(id).Consume(func(msg *tangle.Message) {
		if payload := msg.Payload(); payload != nil && payload.Type() == ledgerstate.TransactionType {
			f(payload.(*ledgerstate.Transaction))
		}
	})
}

// New returns an implementation for waspconn.Ledger
func New() *TangleLedger {
	t := &TangleLedger{}

	t.txConfirmedClosure = events.NewClosure(func(id tangle.MessageID) {
		extractTransaction(id, t.txConfirmedCallback)
	})
	messagelayer.Tangle().ConsensusManager.Events.TransactionConfirmed.Attach(t.txConfirmedClosure)

	t.txBookedClosure = events.NewClosure(func(id tangle.MessageID) {
		extractTransaction(id, t.txBookedCallback)
	})
	messagelayer.Tangle().Booker.Events.MessageBooked.Attach(t.txBookedClosure)

	return t
}

func (t *TangleLedger) Detach() {
	messagelayer.Tangle().ConsensusManager.Events.TransactionConfirmed.Detach(t.txConfirmedClosure)
	messagelayer.Tangle().Booker.Events.MessageBooked.Detach(t.txBookedClosure)
}

func (t *TangleLedger) OnTransactionConfirmed(cb func(tx *ledgerstate.Transaction)) {
	t.txConfirmedCallback = cb
}

func (t *TangleLedger) OnTransactionBooked(cb func(tx *ledgerstate.Transaction)) {
	t.txBookedCallback = cb
}

// GetAddressOutputs returns the available UTXOs for an address
func (t *TangleLedger) GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output)) {
	messagelayer.Tangle().LedgerState.OutputsOnAddress(addr).Consume(func(output ledgerstate.Output) {
		ok := true
		if state, err := t.GetTxInclusionState(output.ID().TransactionID()); err != nil || state != ledgerstate.Confirmed {
			return
		}
		messagelayer.Tangle().LedgerState.OutputMetadata(output.ID()).Consume(func(outputMetadata *ledgerstate.OutputMetadata) {
			if outputMetadata.ConsumerCount() != 0 {
				ok = false
				return
			}
		})
		if ok {
			f(output)
		}
	})
}

// GetConfirmedTransaction fetches a transaction by ID, and executes the given callback if found
func (t *TangleLedger) GetConfirmedTransaction(txid ledgerstate.TransactionID, f func(ret *ledgerstate.Transaction)) (found bool) {
	found = false
	messagelayer.Tangle().LedgerState.Transaction(txid).Consume(func(tx *ledgerstate.Transaction) {
		state, _ := t.GetTxInclusionState(txid)
		if state == ledgerstate.Confirmed {
			found = true
			f(tx)
		}
	})
	return
}

func (ce *TangleLedger) GetTxInclusionState(txid ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	return messagelayer.Tangle().LedgerState.TransactionInclusionState(txid)
}

func (t *TangleLedger) PostTransaction(tx *ledgerstate.Transaction) error {
	_, err := messagelayer.Tangle().IssuePayload(tx)
	if err != nil {
		return fmt.Errorf("failed to issue transaction: %w", err)
	}
	return nil
}

func (t *TangleLedger) RequestFunds(target ledgerstate.Address) error {
	faucetPayload, err := faucet.NewRequest(target, config.Node().Int(faucet.CfgFaucetPoWDifficulty))
	if err != nil {
		return err
	}
	_, err = messagelayer.Tangle().MessageFactory.IssuePayload(faucetPayload, messagelayer.Tangle())
	return err
}
