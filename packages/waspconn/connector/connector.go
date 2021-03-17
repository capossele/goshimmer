package connector

import (
	"io"
	"net"
	"strings"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/waspconn"
	"github.com/iotaledger/goshimmer/packages/waspconn/chopper"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil/buffconn"
)

type WaspConnector struct {
	id                                 string
	bconn                              *buffconn.BufferedConnection
	subscriptions                      map[[ledgerstate.AddressLength]byte]bool
	inTxChan                           chan interface{}
	exitConnChan                       chan struct{}
	receiveConfirmedTransactionClosure *events.Closure
	receiveBookedTransactionClosure    *events.Closure
	receiveWaspMessageClosure          *events.Closure
	messageChopper                     *chopper.Chopper
	log                                *logger.Logger
	vtangle                            waspconn.Ledger
}

type wrapConfirmedTx *ledgerstate.Transaction
type wrapBookedTx *ledgerstate.Transaction

func Run(conn net.Conn, log *logger.Logger, vtangle waspconn.Ledger) {
	wconn := &WaspConnector{
		bconn:          buffconn.NewBufferedConnection(conn, tangle.MaxMessageSize),
		exitConnChan:   make(chan struct{}),
		messageChopper: chopper.NewChopper(),
		log:            log,
		vtangle:        vtangle,
	}
	err := daemon.BackgroundWorker(wconn.Id(), func(shutdownSignal <-chan struct{}) {
		select {
		case <-shutdownSignal:
			wconn.log.Infof("shutdown signal received..")
			_ = wconn.bconn.Close()

		case <-wconn.exitConnChan:
			wconn.log.Infof("closing connection..")
			_ = wconn.bconn.Close()
		}

		wconn.detach()
	}, shutdown.PriorityTangle) // TODO proper priority

	if err != nil {
		close(wconn.exitConnChan)
		wconn.log.Errorf("can't start deamon")
		return
	}
	wconn.attach()
}

func (wconn *WaspConnector) Id() string {
	if wconn.id == "" {
		return "wasp_" + wconn.bconn.RemoteAddr().String()
	}
	return wconn.id
}

func (wconn *WaspConnector) SetId(id string) {
	wconn.id = id
	wconn.log = wconn.log.Named(id)
	wconn.log.Infof("wasp connection id has been set to '%s' for '%s'", id, wconn.bconn.RemoteAddr().String())
}

func (wconn *WaspConnector) attach() {
	wconn.subscriptions = make(map[[ledgerstate.AddressLength]byte]bool)
	wconn.inTxChan = make(chan interface{})

	wconn.receiveConfirmedTransactionClosure = events.NewClosure(func(vtx *ledgerstate.Transaction) {
		wconn.inTxChan <- wrapConfirmedTx(vtx)
	})

	wconn.receiveBookedTransactionClosure = events.NewClosure(func(vtx *ledgerstate.Transaction) {
		wconn.inTxChan <- wrapBookedTx(vtx)
	})

	wconn.receiveWaspMessageClosure = events.NewClosure(func(data []byte) {
		wconn.processMsgDataFromWasp(data)
	})

	// attach connector to the flow of incoming value transactions
	EventValueTransactionConfirmed.Attach(wconn.receiveConfirmedTransactionClosure)
	EventValueTransactionBooked.Attach(wconn.receiveBookedTransactionClosure)

	wconn.bconn.Events.ReceiveMessage.Attach(wconn.receiveWaspMessageClosure)

	wconn.log.Debugf("attached waspconn")

	// read connection thread
	go func() {
		if err := wconn.bconn.Read(); err != nil {
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				wconn.log.Warnw("Permanent error", "err", err)
			}
		}
		close(wconn.exitConnChan)
	}()

	// read incoming pre-filtered transactions from node
	go func() {
		for vtx := range wconn.inTxChan {
			switch tvtx := vtx.(type) {
			case wrapConfirmedTx:
				wconn.processConfirmedTransactionFromNode(tvtx)

			case wrapBookedTx:
				wconn.processBookedTransactionFromNode(tvtx)

			default:
				wconn.log.Panicf("wrong type")
			}
		}
	}()
}

func (wconn *WaspConnector) detach() {
	EventValueTransactionConfirmed.Detach(wconn.receiveConfirmedTransactionClosure)
	EventValueTransactionBooked.Detach(wconn.receiveBookedTransactionClosure)
	wconn.bconn.Events.ReceiveMessage.Detach(wconn.receiveWaspMessageClosure)

	wconn.messageChopper.Close()
	close(wconn.inTxChan)
	_ = wconn.bconn.Close()
	wconn.log.Debugf("detached waspconn")
}

func (wconn *WaspConnector) subscribe(addr *ledgerstate.AliasAddress) {
	_, ok := wconn.subscriptions[addr.Array()]
	if !ok {
		wconn.log.Infof("subscribed to address %s", addr.String())
		wconn.subscriptions[addr.Array()] = true
	}
}

func (wconn *WaspConnector) isSubscribed(addr *ledgerstate.AliasAddress) bool {
	_, ok := wconn.subscriptions[addr.Array()]
	return ok
}

func isRequestOrChainOutput(out ledgerstate.Output) bool {
	switch out.(type) {
	case *ledgerstate.ChainOutput:
		return true
	case *ledgerstate.ExtendedLockedOutput:
		return true
	}
	return false
}

func (wconn *WaspConnector) txSubscribedAddresses(tx *ledgerstate.Transaction) []*ledgerstate.AliasAddress {
	ret := make([]*ledgerstate.AliasAddress, 0)
	for _, output := range tx.Essence().Outputs() {
		if !isRequestOrChainOutput(output) {
			continue
		}
		addr, ok := output.Address().(*ledgerstate.AliasAddress)
		if !ok {
			continue
		}
		if wconn.isSubscribed(addr) {
			ret = append(ret, addr)
		}
	}
	return ret
}

// processConfirmedTransactionFromNode receives only confirmed transactions
// it parses SC transaction incoming from the node. Forwards it to Wasp if subscribed
func (wconn *WaspConnector) processConfirmedTransactionFromNode(tx *ledgerstate.Transaction) {
	for _, addr := range wconn.txSubscribedAddresses(tx) {
		wconn.pushTransaction(tx.ID(), addr)
	}
}

func (wconn *WaspConnector) processBookedTransactionFromNode(tx *ledgerstate.Transaction) {
	addrs := wconn.txSubscribedAddresses(tx)
	if len(addrs) == 0 {
		return
	}
	txid := tx.ID()
	wconn.log.Infof("booked tx -> Wasp. txid: %s", tx.ID().String())
	wconn.sendTxInclusionStateToWasp(txid, ledgerstate.Pending)
}

func (wconn *WaspConnector) getTxInclusionState(txid ledgerstate.TransactionID) {
	state, err := wconn.vtangle.GetTxInclusionState(txid)
	if err != nil {
		wconn.log.Errorf("getTxInclusionState: %v", err)
		return
	}
	wconn.sendTxInclusionStateToWasp(txid, state)
}

func (wconn *WaspConnector) getBacklog(addr *ledgerstate.AliasAddress) {
	var txs map[ledgerstate.TransactionID]bool
	wconn.vtangle.GetUnspentOutputs(addr, func(out ledgerstate.Output) {
		txid := out.ID().TransactionID()
		if isRequestOrChainOutput(out) {
			txs[txid] = true
		}
	})
	for txid := range txs {
		wconn.pushTransaction(txid, addr)
	}
}

func (wconn *WaspConnector) postTransaction(tx *ledgerstate.Transaction) {
	if err := wconn.vtangle.PostTransaction(tx); err != nil {
		wconn.log.Warnf("%v: %s", err, tx.ID().String())
	}
}
