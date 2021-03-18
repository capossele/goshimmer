package waspconn

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/marshalutil"
)

type MessageType byte

const (
	waspMsgChunk = MessageType(iota)

	// wasp -> node
	waspToNodePostTransaction
	waspToNodeSubscribe
	waspToNodeGetConfirmedTransaction
	waspToNodeGetTxInclusionState
	waspToNodeGetBacklog
	waspToNodeSetId

	// node -> wasp
	waspFromNodeTransaction
	waspFromNodeTxInclusionState
)

const ChunkMessageHeaderSize = 3

type Message interface {
	Write(w *marshalutil.MarshalUtil)
	Read(r *marshalutil.MarshalUtil) error
	Type() MessageType
}

// special messages for big Data packets chopped into pieces
type WaspMsgChunk struct {
	Data []byte
}

// Wasp --> Goshimmer

// WaspToNodePostTransactionMsg Wasp nodes publishes state update transaction
type WaspToNodePostTransactionMsg struct {
	Tx *ledgerstate.Transaction
}

// WaspToNodeSubscribeMsg wasp node subscribes to requests/transactions for the chain(s)
type WaspToNodeSubscribeMsg struct {
	ChainAddresses []*ledgerstate.AliasAddress
}

// WaspToNodeGetConfirmedTransactionMsg wasp asks (pulls) specific transaction from goshimmer
// Only asks for confirmed tx
type WaspToNodeGetConfirmedTransactionMsg struct {
	ChainAddress *ledgerstate.AliasAddress
	TxID         ledgerstate.TransactionID
}

// WaspToNodeGetTxInclusionStateMsg wasp node asks (pulls) inclusion state for specific transaction
// in the chain. Normally wasp is waiting for the confirmation of transaction, so after some timeout
// it asks (puls) its state. The chain address is a return address for dispatcher
type WaspToNodeGetTxInclusionStateMsg struct {
	ChainAddress *ledgerstate.AliasAddress
	TxID         ledgerstate.TransactionID
}

// WaspToNodeGetBacklogMsg wasp asks (pulls) the backlog for the chain
type WaspToNodeGetBacklogMsg struct {
	ChainAddress *ledgerstate.AliasAddress
}

// WaspToNodeSetIdMsg wasp informs the node about its ID (mostly for tracing/loging)
type WaspToNodeSetIdMsg struct {
	WaspID string
}

// Goshimmer --> Wasp

type WaspFromNodeTransactionMsg struct {
	// ChainAddress is the chain address of interest (subscribed)
	ChainAddress *ledgerstate.AliasAddress
	// Tx is the transaction being sent
	Tx *ledgerstate.Transaction
	// Sender is the unique sender of the transaction
	Sender ledgerstate.Address
}

// WaspFromNodeTxInclusionStateMsg update to the transaction status
type WaspFromNodeTxInclusionStateMsg struct {
	TxID  ledgerstate.TransactionID
	State ledgerstate.InclusionState
}

/////////////////////////////////

func EncodeMsg(msg Message) []byte {
	m := marshalutil.New()
	m.WriteByte(byte(msg.Type()))
	msg.Write(m)
	return m.Bytes()
}

func DecodeMsg(data []byte, waspSide bool) (interface{}, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("wrong message")
	}
	var ret Message

	switch MessageType(data[0]) {
	case waspMsgChunk:
		ret = &WaspMsgChunk{}

	case waspToNodePostTransaction:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodePostTransactionMsg{}

	case waspToNodeSubscribe:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeSubscribeMsg{}

	case waspToNodeGetConfirmedTransaction:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeGetConfirmedTransactionMsg{}

	case waspToNodeGetTxInclusionState:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeGetTxInclusionStateMsg{}

	case waspToNodeGetBacklog:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeGetBacklogMsg{}

	case waspToNodeSetId:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeSetIdMsg{}

	case waspFromNodeTransaction:
		if !waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspFromNodeTransactionMsg{}

	case waspFromNodeTxInclusionState:
		if !waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspFromNodeTxInclusionStateMsg{}

	default:
		return nil, fmt.Errorf("wrong message code")
	}
	if err := ret.Read(marshalutil.New(data[1:])); err != nil {
		return nil, err
	}
	return ret, nil
}

func (msg *WaspToNodePostTransactionMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Tx)
}

func (msg *WaspToNodePostTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Tx, err = ledgerstate.TransactionFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspToNodePostTransactionMsg) Type() MessageType {
	return waspToNodePostTransaction
}

func (msg *WaspToNodeSubscribeMsg) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.ChainAddresses)))
	for _, addr := range msg.ChainAddresses {
		w.Write(addr)
	}
}

func (msg *WaspToNodeSubscribeMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.ChainAddresses = make([]*ledgerstate.AliasAddress, size)
	for i := uint16(0); i < size; i++ {
		if msg.ChainAddresses[i], err = ledgerstate.AliasAddressFromMarshalUtil(m); err != nil {
			return err
		}
	}
	return nil
}

func (msg *WaspToNodeSubscribeMsg) Type() MessageType {
	return waspToNodeSubscribe
}

func (msg *WaspToNodeGetConfirmedTransactionMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.ChainAddress)
	w.Write(msg.TxID)
}

func (msg *WaspToNodeGetConfirmedTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.ChainAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m); err != nil {
		return err
	}
	msg.TxID, err = ledgerstate.TransactionIDFromMarshalUtil(m)
	return err
}

func (msg *WaspToNodeGetConfirmedTransactionMsg) Type() MessageType {
	return waspToNodeGetConfirmedTransaction
}

func (msg *WaspToNodeGetTxInclusionStateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.TxID)
}

func (msg *WaspToNodeGetTxInclusionStateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.TxID, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspToNodeGetTxInclusionStateMsg) Type() MessageType {
	return waspToNodeGetTxInclusionState
}

func (msg *WaspToNodeGetBacklogMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.ChainAddress)
}

func (msg *WaspToNodeGetBacklogMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	msg.ChainAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m)
	return err
}

func (msg *WaspToNodeGetBacklogMsg) Type() MessageType {
	return waspToNodeGetBacklog
}

func (msg *WaspToNodeSetIdMsg) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.WaspID)))
	w.WriteBytes([]byte(msg.WaspID))
}

func (msg *WaspToNodeSetIdMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	var waspID []byte
	if waspID, err = m.ReadBytes(int(size)); err != nil {
		return err
	}
	msg.WaspID = string(waspID)
	return nil
}

func (msg *WaspToNodeSetIdMsg) Type() MessageType {
	return waspToNodeSetId
}

func (msg *WaspFromNodeTransactionMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.ChainAddress)
	w.Write(msg.Tx)
	w.Write(msg.Sender)
}

func (msg *WaspFromNodeTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.ChainAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.Tx, err = ledgerstate.TransactionFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.Sender, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspFromNodeTransactionMsg) Type() MessageType {
	return waspFromNodeTransaction
}

func (msg *WaspMsgChunk) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.Data)))
	w.WriteBytes(msg.Data)
}

func (msg *WaspMsgChunk) Read(m *marshalutil.MarshalUtil) error {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.Data, err = m.ReadBytes(int(size))
	return err
}

func (msg *WaspMsgChunk) Type() MessageType {
	return waspMsgChunk
}

func (msg *WaspFromNodeTxInclusionStateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.State)
	w.Write(msg.TxID)
}

func (msg *WaspFromNodeTxInclusionStateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.State, err = ledgerstate.InclusionStateFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.TxID, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspFromNodeTxInclusionStateMsg) Type() MessageType {
	return waspFromNodeTxInclusionState
}
