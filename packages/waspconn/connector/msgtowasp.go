package connector

// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/ledgerstate/utxoutil"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/waspconn"
)

func (wconn *WaspConnector) sendMsgToWasp(msg waspconn.Message) {
	var err error
	defer func() {
		if err != nil {
			wconn.log.Errorf("sendMsgToWasp: %s", err.Error())
		}
	}()

	data := waspconn.EncodeMsg(msg)
	choppedData, chopped, err := wconn.messageChopper.ChopData(data, tangle.MaxMessageSize, waspconn.ChunkMessageHeaderSize)
	if err != nil {
		return
	}
	if !chopped {
		_, err = wconn.bconn.Write(data)
		return
	}

	// sending piece by piece wrapped in WaspMsgChunk
	for _, piece := range choppedData {
		dataToSend := waspconn.EncodeMsg(&waspconn.WaspMsgChunk{
			Data: piece,
		})
		if len(dataToSend) > tangle.MaxMessageSize {
			wconn.log.Panicf("sendMsgToWasp: internal inconsistency 3 size too big: %d", len(dataToSend))
		}
		_, err = wconn.bconn.Write(dataToSend)
		if err != nil {
			return
		}
	}
}

func (wconn *WaspConnector) sendTxInclusionStateToWasp(txid ledgerstate.TransactionID, state ledgerstate.InclusionState) {
	wconn.sendMsgToWasp(&waspconn.WaspFromNodeTxInclusionStateMsg{
		TxID:  txid,
		State: state,
	})
}

func (wconn *WaspConnector) pushTransaction(txid ledgerstate.TransactionID, chainAddress *ledgerstate.AliasAddress) {
	found := wconn.vtangle.GetConfirmedTransaction(txid, func(tx *ledgerstate.Transaction) {
		sender, err := wconn.fetchSender(tx)
		if err != nil {
			wconn.log.Errorf("fetchSender: %s", err.Error())
			return
		}
		wconn.sendMsgToWasp(&waspconn.WaspFromNodeTransactionMsg{
			ChainAddress: chainAddress,
			Tx:           tx,
			Sender:       sender,
		})
	})
	if !found {
		wconn.log.Warnf("pushTransaction: not found %s", txid.String())
	}
}

func (wconn *WaspConnector) fetchSender(tx *ledgerstate.Transaction) (ledgerstate.Address, error) {

	// TODO no need anymore for 'sender' included into the package with a transaction
	//inputs, err := wconn.fetchInputs(tx.Essence().Inputs())
	//if err != nil {
	//	return nil, err
	//}
	return utxoutil.GetSingleSender(tx)
}

func (wconn *WaspConnector) fetchInputs(outs []ledgerstate.Input) ([]ledgerstate.Output, error) {
	inputs := make([]ledgerstate.Output, len(outs))
	for i, input := range outs {
		utxoInput, ok := input.(*ledgerstate.UTXOInput)
		if !ok {
			wconn.log.Debugf("fetchInputs: unknown output type: %T", input)
			continue
		}
		wconn.vtangle.GetConfirmedTransaction(utxoInput.ReferencedOutputID().TransactionID(), func(tx *ledgerstate.Transaction) {
			txOuts := tx.Essence().Outputs()
			outIndex := utxoInput.ReferencedOutputID().OutputIndex()
			if len(txOuts) < int(outIndex) {
				wconn.log.Debugf("fetchInputs: invalid output index %d for tx %s", outIndex, tx.ID())
				return
			}
			inputs[i] = tx.Essence().Outputs()[outIndex].Clone()
		})
	}
	return inputs, nil
}
