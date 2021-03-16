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
	waspToNodeTransaction
	waspToNodeSubscribe
	waspToNodeGetConfirmedTransaction
	waspToNodeGetBranchInclusionState
	waspToNodeGetOutputs
	waspToNodeSetId

	// node -> wasp
	waspFromNodeConfirmedTransaction
	waspFromNodeBacklogUpdate
	waspFromNodeBranchInclusionState
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

// WaspToNodeTransactionMsg Wasp nodes publishes state update transaction
type WaspToNodeTransactionMsg struct {
	Tx           *ledgerstate.Transaction  // transaction posted
	ChainAddress *ledgerstate.AliasAddress // mostly for logging/testing
	Leader       uint16                    // mostly for logging/testing
}

// WaspToNodeSubscribeMsg wasp node subscribes to requests/transactions for the chain(s)
type WaspToNodeSubscribeMsg struct {
	ChainAddresses []*ledgerstate.AliasAddress
}

// WaspToNodeGetConfirmedTransactionMsg wasp asks (pulls) specific transaction from goshimmer
// Only asks for confirmed tx
type WaspToNodeGetConfirmedTransactionMsg struct {
	TxId ledgerstate.TransactionID
}

// WaspToNodeGetBranchInclusionStateMsg wasp node asks (pulls) inclusion state for specific transaction
// in the chain. Normally wasp is waiting for the confirmation of transaction, so after some timeout
// it asks (puls) its state. The chain address is a return address for dispatcher
type WaspToNodeGetBranchInclusionStateMsg struct {
	TxId         ledgerstate.TransactionID
	ChainAddress *ledgerstate.AliasAddress
}

// WaspToNodeGetOutputsMsg wasp asks (pulls) the backlog for the chain
type WaspToNodeGetOutputsMsg struct {
	ChainAddress *ledgerstate.AliasAddress
}

// WaspToNodeSetIdMsg wasp informs the node about its ID (mostly for tracing/loging)
type WaspToNodeSetIdMsg struct {
	Waspid string
}

// Goshimmer --> Wasp

// NEW TEMPORARY

// May be enough to have this one structure to update the information about backlog

type WaspFromNodeAccountUpdate struct {
	// TxId is mandatory id of transaction where information has been collected from
	TxId ledgerstate.TransactionID
	// ChainAddress is mandatory chain address of interest (subscribed), the filter for the info
	ChainAddress *ledgerstate.AliasAddress
	// Sender is a unique sender of the transaction of TxId
	Sender *ledgerstate.Address
	// MintProofs mint proofs present in the transaction TxId
	MintProofs map[ledgerstate.Color]uint64
	// optional. Only if chain output is present in the tx. Must have address == ChainAddress
	ChainOutput *ledgerstate.ChainOutput
	// optional. Lists all outputs in the tx with the address of ChainAddress
	Outputs []*ledgerstate.ExtendedLockedOutput
}

/////////////////////////////////

// WaspFromNodeConfirmedTransactionMsg node sends (push) to wasp a confirmed transaction of interest
// (with subscribed outputs).
type WaspFromNodeConfirmedTransactionMsg struct {
	Sender ledgerstate.Address
	Tx     *ledgerstate.Transaction
}

// WaspFromNodeBacklogUpdateMsg update of the backlog
type WaspFromNodeBacklogUpdateMsg struct {
	ChainOutput *ledgerstate.ChainOutput // must exists always. chain address is known from this one
	Outputs     []*ledgerstate.ExtendedLockedOutput
	Senders     map[ledgerstate.TransactionID]ledgerstate.Address
}

// WaspFromNodeBranchInclusionStateMsg update to the transaction status
type WaspFromNodeBranchInclusionStateMsg struct {
	State ledgerstate.InclusionState
	TxId  ledgerstate.TransactionID
}

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

	case waspToNodeTransaction:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeTransactionMsg{}

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

	case waspToNodeGetBranchInclusionState:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeGetBranchInclusionStateMsg{}

	case waspToNodeGetOutputs:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeGetOutputsMsg{}

	case waspToNodeSetId:
		if waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspToNodeSetIdMsg{}

	case waspFromNodeConfirmedTransaction:
		if !waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspFromNodeConfirmedTransactionMsg{}

	case waspFromNodeBacklogUpdate:
		if !waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspFromNodeBacklogUpdateMsg{}

	case waspFromNodeBranchInclusionState:
		if !waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspFromNodeBranchInclusionStateMsg{}

	default:
		return nil, fmt.Errorf("wrong message code")
	}
	if err := ret.Read(marshalutil.New(data[1:])); err != nil {
		return nil, err
	}
	return ret, nil
}

func (msg *WaspToNodeTransactionMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Tx)
	w.Write(msg.ChainAddress)
	w.WriteUint16(msg.Leader)
}

func (msg *WaspToNodeTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Tx, err = ledgerstate.TransactionFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.ChainAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.Leader, err = m.ReadUint16(); err != nil {
		return err
	}
	return nil
}

func (msg *WaspToNodeTransactionMsg) Type() MessageType {
	return waspToNodeTransaction
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
	w.Write(msg.TxId)
}

func (msg *WaspToNodeGetConfirmedTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	msg.TxId, err = ledgerstate.TransactionIDFromMarshalUtil(m)
	return err
}

func (msg *WaspToNodeGetConfirmedTransactionMsg) Type() MessageType {
	return waspToNodeGetConfirmedTransaction
}

func (msg *WaspToNodeGetBranchInclusionStateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.TxId)
	w.Write(msg.ChainAddress)
}

func (msg *WaspToNodeGetBranchInclusionStateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.TxId, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.ChainAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspToNodeGetBranchInclusionStateMsg) Type() MessageType {
	return waspToNodeGetBranchInclusionState
}

func (msg *WaspToNodeGetOutputsMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.ChainAddress)
}

func (msg *WaspToNodeGetOutputsMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	msg.ChainAddress, err = ledgerstate.AliasAddressFromMarshalUtil(m)
	return err
}

func (msg *WaspToNodeGetOutputsMsg) Type() MessageType {
	return waspToNodeGetOutputs
}

func (msg *WaspToNodeSetIdMsg) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.Waspid)))
	w.WriteBytes([]byte(msg.Waspid))
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
	msg.Waspid = string(waspID)
	return nil
}

func (msg *WaspToNodeSetIdMsg) Type() MessageType {
	return waspToNodeSetId
}

func (msg *WaspFromNodeConfirmedTransactionMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Tx)
}

func (msg *WaspFromNodeConfirmedTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Tx, err = ledgerstate.TransactionFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspFromNodeConfirmedTransactionMsg) Type() MessageType {
	return waspFromNodeConfirmedTransaction
}

func (msg *WaspFromNodeBacklogUpdateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.ChainOutput)
	w.WriteUint16(uint16(len(msg.Outputs)))
	for _, out := range msg.Outputs {
		w.Write(out)
	}
	w.WriteUint16(uint16(len(msg.Senders)))
	for txid, addr := range msg.Senders {
		w.Write(txid)
		w.Write(addr)
	}
}

func (msg *WaspFromNodeBacklogUpdateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.ChainOutput, err = ledgerstate.ChainOutputFromMarshalUtil(m); err != nil {
		return err
	}
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.Outputs = make([]*ledgerstate.ExtendedLockedOutput, size)
	for i := uint16(0); i < size; i++ {
		if msg.Outputs[i], err = ledgerstate.ExtendedOutputFromMarshalUtil(m); err != nil {
			return err
		}
	}
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.Senders = make(map[ledgerstate.TransactionID]ledgerstate.Address, size)
	for i := uint16(0); i < size; i++ {
		var txid ledgerstate.TransactionID
		if txid, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
			return err
		}
		if msg.Senders[txid], err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
			return err
		}
	}
	return nil
}

func (msg *WaspFromNodeBacklogUpdateMsg) Type() MessageType {
	return waspFromNodeBacklogUpdate
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

func (msg *WaspFromNodeBranchInclusionStateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.State)
	w.Write(msg.TxId)
}

func (msg *WaspFromNodeBranchInclusionStateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.State, err = ledgerstate.InclusionStateFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.TxId, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspFromNodeBranchInclusionStateMsg) Type() MessageType {
	return waspFromNodeBranchInclusionState
}
