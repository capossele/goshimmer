package utxodb

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxoutil"
	"golang.org/x/xerrors"
)

// IsConfirmed checks if the transaction is in the UTXODB (in the ledger)
func (u *UtxoDB) IsConfirmed(txid *ledgerstate.TransactionID) bool {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	_, ok := u.transactions[*txid]
	return ok
}

// PostTransaction adds a transaction to UTXODB or returns an error.
// The function ensures consistency of the UTXODB ledger
func (u *UtxoDB) PostTransaction(tx *ledgerstate.Transaction) error {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	// serialize/deserialize for proper semantic check
	tx, _, err := ledgerstate.TransactionFromBytes(tx.Bytes())
	if err != nil {
		return err
	}
	if err := u.CheckTransaction(tx); err != nil {
		return err
	}
	// delete consumed (referenced) outputs from the ledger
	for _, inp := range tx.Essence().Inputs() {
		utxoInp := inp.(*ledgerstate.UTXOInput)

		consumed, ok := u.findUnspentOutputByID(utxoInp.ReferencedOutputID())
		if !ok {
			return xerrors.New("deleting UTXO: corresponding output does not exists")
		}
		delete(u.utxo, utxoInp.ReferencedOutputID())
		u.consumedOutputs[utxoInp.ReferencedOutputID()] = consumed
	}
	// add outputs to the ledger
	for _, out := range tx.Essence().Outputs() {
		if out.ID().TransactionID() != tx.ID() {
			panic("utxodb.AddTransaction: incorrect output ID")
		}
		u.utxo[out.ID()] = out.UpdateMintingColor()
	}
	u.transactions[tx.ID()] = tx
	u.checkLedgerBalance()

	if u.txConfirmedCallback != nil {
		u.txConfirmedCallback(tx)
	}

	return nil
}

// findOutputByID return outputs, (true/false == unspent/consumed), error
func (u *UtxoDB) findUnspentOutputByID(id ledgerstate.OutputID) (ledgerstate.Output, bool) {
	if out, ok := u.utxo[id]; ok {
		return out, true
	}
	return nil, false
}

// findOutputByID return outputs, (true/false == unspent/consumed), error
func (u *UtxoDB) findSpentOutputByID(id ledgerstate.OutputID) (ledgerstate.Output, bool) {
	if out, ok := u.consumedOutputs[id]; ok {
		return out, true
	}
	return nil, false
}

// GetTransaction retrieves value transaction by its hash (ID)
func (u *UtxoDB) GetTransaction(id ledgerstate.TransactionID) (*ledgerstate.Transaction, bool) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getTransaction(id)
}

// GetConfirmedTransaction is the same as GetTransaction, but implementing the waspconn.Ledger interface
func (u *UtxoDB) GetConfirmedTransaction(id ledgerstate.TransactionID, f func(tx *ledgerstate.Transaction)) bool {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	tx, ok := u.getTransaction(id)
	if ok {
		f(tx)
	}
	return ok
}

// GetTxInclusionState returns the inclusion state of the tx by id
func (u *UtxoDB) GetTxInclusionState(txid ledgerstate.TransactionID) (ledgerstate.InclusionState, error) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	_, ok := u.getTransaction(txid)
	if ok {
		return ledgerstate.Confirmed, nil
	}
	return 0, xerrors.Errorf("tx not found")
}

// MustGetTransaction same as GetTransaction only panics if transaction is not in UTXODB
func (u *UtxoDB) MustGetTransaction(id ledgerstate.TransactionID) *ledgerstate.Transaction {
	u.mutex.RLock()
	defer u.mutex.RUnlock()
	return u.mustGetTransaction(id)
}

// GetAddressOutputs returns outputs contained in the address
func (u *UtxoDB) GetAddressOutputs(addr ledgerstate.Address) []ledgerstate.Output {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	return u.getAddressOutputs(addr)
}

// GetUnspentOutputs is the same as GetAddressOutputs, but implementing the waspconn.Ledger interface
func (u *UtxoDB) GetUnspentOutputs(addr ledgerstate.Address, f func(output ledgerstate.Output)) {
	u.mutex.RLock()
	defer u.mutex.RUnlock()

	for _, out := range u.getAddressOutputs(addr) {
		f(out)
	}
}

// GetAddressBalances return all colored balances of the address
func (u *UtxoDB) GetAddressBalances(addr ledgerstate.Address) map[ledgerstate.Color]uint64 {
	ret := make(map[ledgerstate.Color]uint64)
	outputs := u.GetAddressOutputs(addr)
	for _, out := range outputs {
		out.Balances().ForEach(func(col ledgerstate.Color, bal uint64) bool {
			s, _ := ret[col]
			ret[col] = s + bal
			return true
		})
	}
	return ret
}

func (u *UtxoDB) Balance(addr ledgerstate.Address, color ledgerstate.Color) uint64 {
	bals := u.GetAddressBalances(addr)
	ret, _ := bals[color]
	return ret
}

func (u *UtxoDB) BalanceIOTA(addr ledgerstate.Address) uint64 {
	return u.Balance(addr, ledgerstate.ColorIOTA)
}

func (u *UtxoDB) getTransaction(id ledgerstate.TransactionID) (*ledgerstate.Transaction, bool) {
	tx, ok := u.transactions[id]
	return tx, ok
}

func (u *UtxoDB) mustGetTransaction(id ledgerstate.TransactionID) *ledgerstate.Transaction {
	tx, ok := u.transactions[id]
	if !ok {
		panic(xerrors.Errorf("utxodb.mustGetTransaction: tx id doesn't exist: %s", id.String()))
	}
	return tx
}

func (u *UtxoDB) getAddressOutputs(addr ledgerstate.Address) []ledgerstate.Output {
	addrArr := addr.Array()
	ret := make([]ledgerstate.Output, 0)
	for _, out := range u.utxo {
		if out.Address().Array() == addrArr {
			ret = append(ret, out)
		}
	}
	return ret
}

func (u *UtxoDB) getOutputTotal(outid ledgerstate.OutputID) (uint64, error) {
	out, ok := u.utxo[outid]
	if !ok {
		return 0, xerrors.Errorf("no such output: %s", outid.String())
	}
	ret := uint64(0)
	out.Balances().ForEach(func(_ ledgerstate.Color, bal uint64) bool {
		ret += bal
		return true
	})
	return ret, nil
}

func (u *UtxoDB) checkLedgerBalance() {
	total := uint64(0)
	for outp := range u.utxo {
		b, err := u.getOutputTotal(outp)
		if err != nil {
			panic("utxodb: wrong ledger balance: " + err.Error())
		}
		total += b
	}
	if total != Supply {
		panic("utxodb: wrong ledger balance")
	}
}

func (u *UtxoDB) collectUnspentOutputsFromInputs(tx *ledgerstate.Transaction) ([]ledgerstate.Output, error) {
	ret := make([]ledgerstate.Output, len(tx.Essence().Inputs()))
	for i, inp := range tx.Essence().Inputs() {
		if inp.Type() != ledgerstate.UTXOInputType {
			return nil, xerrors.New("collectUnspentOutputsFromInputs: wrong input type")
		}
		utxoInp := inp.(*ledgerstate.UTXOInput)
		var ok bool
		oid := utxoInp.ReferencedOutputID()
		if ret[i], ok = u.findUnspentOutputByID(oid); !ok {
			return nil, xerrors.New("collectUnspentOutputsFromInputs: unspent output does not exist")
		}
		otx, ok := u.getTransaction(oid.TransactionID())
		if !ok {
			return nil, xerrors.Errorf("collectUnspentOutputsFromInputs: input transaction not found: %s", oid.TransactionID())
		}
		if tx.Essence().Timestamp().Before(otx.Essence().Timestamp()) {
			return nil, xerrors.Errorf("collectUnspentOutputsFromInputs: transaction timestamp is before input timestamp: %s", oid.TransactionID())
		}
	}
	return ret, nil
}

func (u *UtxoDB) collectSpentOutputsFromInputs(tx *ledgerstate.Transaction) ([]ledgerstate.Output, error) {
	ret := make([]ledgerstate.Output, len(tx.Essence().Inputs()))
	for i, inp := range tx.Essence().Inputs() {
		if inp.Type() != ledgerstate.UTXOInputType {
			return nil, xerrors.New("collectSpentOutputsFromInputs: wrong input type")
		}
		utxoInp := inp.(*ledgerstate.UTXOInput)
		var ok bool
		oid := utxoInp.ReferencedOutputID()
		if ret[i], ok = u.findSpentOutputByID(oid); !ok {
			return nil, xerrors.New("collectSpentOutputsFromInputs: spent output does not exist")
		}
		otx, ok := u.getTransaction(oid.TransactionID())
		if !ok {
			return nil, xerrors.Errorf("collectSpentOutputsFromInputs: input transaction not found: %s", oid.TransactionID())
		}
		if tx.Essence().Timestamp().Before(otx.Essence().Timestamp()) {
			return nil, xerrors.Errorf("collectSpentOutputsFromInputs: transaction timestamp is before input timestamp: %s", oid.TransactionID())
		}
	}
	return ret, nil
}

// CollectOutputsFromInputs return spent outputs if transaction is in the ledger,
// otherwise returns unspent inputs or error if does not exist
func (u *UtxoDB) CollectOutputsFromInputs(tx *ledgerstate.Transaction) ([]ledgerstate.Output, bool, error) {
	_, inTheLedger := u.transactions[tx.ID()]
	if inTheLedger {
		outs, err := u.collectSpentOutputsFromInputs(tx)
		if err != nil {
			return nil, true, err
		}
		return outs, true, nil
	}
	// not in the ledger
	outs, err := u.collectUnspentOutputsFromInputs(tx)
	if err != nil {
		return nil, false, err
	}
	return outs, false, nil
}

// CheckTransaction checks consistency of the transaction the same way as ledgerstate
func (u *UtxoDB) CheckTransaction(tx *ledgerstate.Transaction) error {
	inputs, err := u.collectUnspentOutputsFromInputs(tx)
	if err != nil {
		return err
	}
	if !ledgerstate.TransactionBalancesValid(inputs, tx.Essence().Outputs()) {
		return xerrors.Errorf("sum of consumed and spent balances is not 0")
	}
	if ok, err := ledgerstate.UnlockBlocksValidWithError(inputs, tx); !ok || err != nil {
		return xerrors.Errorf("input unlocking failed. Error: %v", err)
	}
	return nil
}

func (u *UtxoDB) GetSingleSender(tx *ledgerstate.Transaction) (ledgerstate.Address, error) {
	inputs, _, err := u.CollectOutputsFromInputs(tx)
	if err != nil {
		return nil, err
	}
	return utxoutil.GetSingleSender(tx, inputs)
}

func (u *UtxoDB) GetChainOutputs(addr ledgerstate.Address) []*ledgerstate.ChainOutput {
	outs := u.GetAddressOutputs(addr)
	ret := make([]*ledgerstate.ChainOutput, 0)
	for _, out := range outs {
		if o, ok := out.(*ledgerstate.ChainOutput); ok {
			ret = append(ret, o)
		}
	}
	return ret
}

func (u *UtxoDB) OnTransactionConfirmed(f func(tx *ledgerstate.Transaction)) {
	u.txConfirmedCallback = f
}

func (u *UtxoDB) OnTransactionBooked(f func(tx *ledgerstate.Transaction)) {
}

func (u *UtxoDB) Detach() {
}