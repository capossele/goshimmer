package utxodb

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/stretchr/testify/require"
)

func TestBasic(t *testing.T) {
	u := New()
	genTx, ok := u.GetTransaction(u.genesisTxId)
	require.True(t, ok)
	require.EqualValues(t, genTx.ID(), u.genesisTxId)
}

func TestGenesis(t *testing.T) {
	u := New()
	require.EqualValues(t, Supply, u.BalanceIOTA(u.GetGenesisAddress()))
	u.checkLedgerBalance()
}

func TestRequestFunds(t *testing.T) {
	u := New()
	_, addr := NewKeyPairByIndex(2)
	err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, Supply-RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr))
	u.checkLedgerBalance()
}

func TestAddTransactionFail(t *testing.T) {
	u := New()
	_, addr := NewKeyPairByIndex(2)
	err := u.RequestFunds(addr)
	require.NoError(t, err)
	require.EqualValues(t, Supply-RequestFundsAmount, u.BalanceIOTA(u.GetGenesisAddress()))
	require.EqualValues(t, RequestFundsAmount, u.BalanceIOTA(addr))
	u.checkLedgerBalance()

	var tx *ledgerstate.Transaction
	for _, out := range u.GetAddressOutputs(addr) {
		tx, _ = u.GetTransaction(out.ID().TransactionID())
	}

	err = u.PostTransaction(tx)
	require.Error(t, err)
	u.checkLedgerBalance()
}
