package waspconn

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/marshalutil"
)

const (
	waspPing = iota
	waspMsgChunk

	// wasp -> node
	waspToNodeTransaction
	waspToNodeSubscribe
	waspToNodeGetConfirmedTransaction
	waspToNodeGetBranchInclusionState
	waspToNodeGetOutputs
	waspToNodeSetId

	// node -> wasp
	waspFromNodeConfirmedTransaction
	waspFromNodeAddressUpdate
	waspFromNodeAddressOutputs
	waspFromNodeBranchInclusionState
)

const ChunkMessageHeaderSize = 3

type Message interface {
	Write(w *marshalutil.MarshalUtil)
	Read(r *marshalutil.MarshalUtil) error
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
	ChainAddress []*ledgerstate.AliasAddress
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

func typeToCode(msg Message) byte {
	switch msg.(type) {
	case *WaspPingMsg:
		return waspPing

	case *WaspMsgChunk:
		return waspMsgChunk

	case *WaspToNodeTransactionMsg:
		return waspToNodeTransaction

	case *WaspToNodeSubscribeMsg:
		return waspToNodeSubscribe

	case *WaspToNodeGetConfirmedTransactionMsg:
		return waspToNodeGetConfirmedTransaction

	case *WaspToNodeGetBranchInclusionStateMsg:
		return waspToNodeGetBranchInclusionState

	case *WaspToNodeGetOutputsMsg:
		return waspToNodeGetOutputs

	case *WaspToNodeSetIdMsg:
		return waspToNodeSetId

	case *WaspFromNodeConfirmedTransactionMsg:
		return waspFromNodeConfirmedTransaction

	case *WaspFromNodeAddressUpdateMsg:
		return waspFromNodeAddressUpdate

	case *WaspFromNodeAddressOutputsMsg:
		return waspFromNodeAddressOutputs

	case *WaspFromNodeBranchInclusionStateMsg:
		return waspFromNodeBranchInclusionState
	}

	panic("wrong type")
}

func EncodeMsg(msg Message) []byte {
	m := marshalutil.New()
	m.WriteByte(typeToCode(msg))
	msg.Write(m)
	return m.Bytes()
}

func DecodeMsg(data []byte, waspSide bool) (interface{}, error) {
	if len(data) < 1 {
		return nil, fmt.Errorf("wrong message")
	}
	var ret Message

	switch data[0] {
	case waspPing:
		ret = &WaspPingMsg{}

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

	case waspFromNodeAddressUpdate:
		if !waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspFromNodeAddressUpdateMsg{}

	case waspFromNodeAddressOutputs:
		if !waspSide {
			return nil, fmt.Errorf("wrong message")
		}
		ret = &WaspFromNodeAddressOutputsMsg{}

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

func (msg *WaspPingMsg) Write(m *marshalutil.MarshalUtil) {
	m.WriteUint32(msg.Id)
	m.WriteTime(msg.Timestamp)
}

func (msg *WaspPingMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Id, err = m.ReadUint32(); err != nil {
		return err
	}
	if msg.Timestamp, err = m.ReadTime(); err != nil {
		return err
	}
	return nil
}

func (msg *WaspToNodeTransactionMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Tx)
	w.Write(msg.SCAddress)
	w.WriteUint16(msg.Leader)
}

func (msg *WaspToNodeTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Tx, err = ledgerstate.TransactionFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.SCAddress, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.Leader, err = m.ReadUint16(); err != nil {
		return err
	}
	return nil
}

func (msg *WaspToNodeSubscribeMsg) Write(w *marshalutil.MarshalUtil) {
	w.WriteUint16(uint16(len(msg.AddressesWithColors)))
	for _, addrCol := range msg.AddressesWithColors {
		w.Write(addrCol.Address)
		w.Write(addrCol.Color)
	}
}

func (msg *WaspToNodeSubscribeMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.AddressesWithColors = make([]AddressColor, size)
	for i := range msg.AddressesWithColors {
		if msg.AddressesWithColors[i].Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
			return err
		}
		if msg.AddressesWithColors[i].Color, err = ledgerstate.ColorFromMarshalUtil(m); err != nil {
			return err
		}
	}
	return nil
}

func (msg *WaspToNodeGetConfirmedTransactionMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.TxId)
}

func (msg *WaspToNodeGetConfirmedTransactionMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	msg.TxId, err = ledgerstate.TransactionIDFromMarshalUtil(m)
	return err
}

func (msg *WaspToNodeGetBranchInclusionStateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.TxId)
	w.Write(msg.SCAddress)
}

func (msg *WaspToNodeGetBranchInclusionStateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.TxId, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.SCAddress, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspToNodeGetOutputsMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
}

func (msg *WaspToNodeGetOutputsMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	msg.Address, err = ledgerstate.AddressFromMarshalUtil(m)
	return err
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

func (msg *WaspFromNodeAddressUpdateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	WriteBalances(w, msg.Balances)
	w.Write(msg.Tx)
}

func (msg *WaspFromNodeAddressUpdateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.Balances, err = ReadBalances(m); err != nil {
		return err
	}
	if msg.Tx, err = ledgerstate.TransactionFromMarshalUtil(m); err != nil {
		return err
	}
	return nil
}

func (msg *WaspFromNodeAddressOutputsMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.Address)
	WriteBalances(w, msg.Balances)
}

func (msg *WaspFromNodeAddressOutputsMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.Address, err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
		return err
	}
	msg.Balances, err = ReadBalances(m)
	return err
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

func (msg *WaspFromNodeBranchInclusionStateMsg) Write(w *marshalutil.MarshalUtil) {
	w.Write(msg.State)
	w.Write(msg.TxId)
	w.WriteUint16(uint16(len(msg.SubscribedAddresses)))
	for i := range msg.SubscribedAddresses {
		w.Write(msg.SubscribedAddresses[i])
	}
}

func (msg *WaspFromNodeBranchInclusionStateMsg) Read(m *marshalutil.MarshalUtil) error {
	var err error
	if msg.State, err = ledgerstate.InclusionStateFromMarshalUtil(m); err != nil {
		return err
	}
	if msg.TxId, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
		return err
	}
	var numAddrs uint16
	if numAddrs, err = m.ReadUint16(); err != nil {
		return err
	}
	msg.SubscribedAddresses = make([]ledgerstate.Address, numAddrs)
	for i := range msg.SubscribedAddresses {
		if msg.SubscribedAddresses[i], err = ledgerstate.AddressFromMarshalUtil(m); err != nil {
			return err
		}
	}
	return nil
}

func WriteBalances(w *marshalutil.MarshalUtil, balances map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances) {
	if err := ValidateBalances(balances); err != nil {
		panic(err)
	}
	w.WriteUint16(uint16(len(balances)))
	for txid, bals := range balances {
		w.Write(txid)
		w.Write(bals)
	}
}

func ReadBalances(m *marshalutil.MarshalUtil) (map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances, error) {
	var err error
	var size uint16
	if size, err = m.ReadUint16(); err != nil {
		return nil, err
	}
	ret := make(map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances, size)
	for i := uint16(0); i < size; i++ {
		var txid ledgerstate.TransactionID
		if txid, err = ledgerstate.TransactionIDFromMarshalUtil(m); err != nil {
			return nil, err
		}
		var bals *ledgerstate.ColoredBalances
		if bals, err = ledgerstate.ColoredBalancesFromMarshalUtil(m); err != nil {
			return nil, err
		}
		ret[txid] = bals
	}
	if err := ValidateBalances(ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func OutputsToBalances(outs map[ledgerstate.OutputID]*ledgerstate.ColoredBalances) map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances {
	ret := make(map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances)
	var niltxid ledgerstate.TransactionID

	for outp, bals := range outs {
		if outp.TransactionID() == niltxid {
			panic("outp.TransactionID() == niltxid")
		}
		ret[outp.TransactionID()] = bals
	}
	return ret
}

func OutputBalancesByColor(outs map[ledgerstate.OutputID]*ledgerstate.ColoredBalances) (map[ledgerstate.Color]uint64, uint64) {
	ret := make(map[ledgerstate.Color]uint64)
	var total uint64
	for _, bals := range outs {
		bals.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			if s, ok := ret[color]; !ok {
				ret[color] = balance
			} else {
				ret[color] = s + balance
			}
			total += balance
			return true
		})
	}
	return ret, total
}

func OutputsByTransactionToString(outs map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances) string {
	ret := ""
	for txid, bals := range outs {
		ret += fmt.Sprintf("     %s:\n", txid.String())
		bals.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			ret += fmt.Sprintf("            %s: %d\n", color.String(), balance)
			return true
		})
	}
	return ret
}

func BalancesByColorToString(bals map[ledgerstate.Color]uint64) string {
	ret := ""
	for col, b := range bals {
		ret += fmt.Sprintf("      %s: %d\n", col.String(), b)
	}
	return ret
}

// InclusionStateText return text representation of the code
func InclusionStateText(state ledgerstate.InclusionState) string {
	switch state {
	case ledgerstate.Pending:
		return "pending"
	case ledgerstate.Confirmed:
		return "confirmed"
	case ledgerstate.Rejected:
		return "rejected"
	}
	return "wrong code"
}

func BalancesToString(outs map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances) string {
	if outs == nil {
		return "empty balances"
	}

	txids := make([]ledgerstate.TransactionID, 0, len(outs))
	for txid := range outs {
		txids = append(txids, txid)
	}
	sort.Slice(txids, func(i, j int) bool {
		return bytes.Compare(txids[i][:], txids[j][:]) < 0
	})

	ret := ""
	for _, txid := range txids {
		bals := outs[txid]
		ret += txid.String() + ":\n"
		bals.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			ret += fmt.Sprintf("         %s: %d\n", color.String(), balance)
			return true
		})
	}
	return ret
}

var niltxid ledgerstate.TransactionID

func ValidateBalances(outs map[ledgerstate.TransactionID]*ledgerstate.ColoredBalances) error {
	for txid, bals := range outs {
		if txid == (ledgerstate.TransactionID)(ledgerstate.ColorMint) || txid == niltxid {
			return errors.New("ValidateBalances: wrong txid")
		}
		var err error
		bals.ForEach(func(color ledgerstate.Color, balance uint64) bool {
			if color == ledgerstate.ColorMint {
				err = errors.New("ValidateBalances: can't be ColorMint")
			}
			return err == nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}
