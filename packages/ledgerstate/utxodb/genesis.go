package utxodb

import (
	"math/rand"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
)

const (
	defaultSupply = uint64(2779530283 * 1000 * 1000)
	genesisIndex  = 31415926535

	RequestFundsAmount = 1337 // same as Goshimmer faucet
)

var essenceVersion = ledgerstate.TransactionEssenceVersion(0)

// UtxoDB is the structure which contains all UTXODB transactions and ledger
type UtxoDB struct {
	seed            *ed25519.Seed
	supply          uint64
	genesisKeyPair  *ed25519.KeyPair
	genesisAddress  ledgerstate.Address
	transactions    map[ledgerstate.TransactionID]*ledgerstate.Transaction
	utxo            map[ledgerstate.OutputID]ledgerstate.Output
	consumedOutputs map[ledgerstate.OutputID]ledgerstate.Output
	mutex           *sync.RWMutex
	genesisTxId     ledgerstate.TransactionID
}

// New creates new UTXODB instance
func newUtxodb(seed *ed25519.Seed, supply uint64) *UtxoDB {
	genesisKeyPair := seed.KeyPair(uint64(genesisIndex))
	genesisAddress := ledgerstate.NewED25519Address(genesisKeyPair.PublicKey)
	u := &UtxoDB{
		seed:            seed,
		supply:          supply,
		genesisKeyPair:  genesisKeyPair,
		genesisAddress:  genesisAddress,
		transactions:    make(map[ledgerstate.TransactionID]*ledgerstate.Transaction),
		utxo:            make(map[ledgerstate.OutputID]ledgerstate.Output),
		consumedOutputs: make(map[ledgerstate.OutputID]ledgerstate.Output),
		mutex:           &sync.RWMutex{},
	}
	u.genesisInit()
	return u
}

func New(supply ...uint64) *UtxoDB {
	s := defaultSupply
	if len(supply) > 0 {
		s = supply[0]
	}
	return newUtxodb(ed25519.NewSeed([]byte("EFonzaUz5ngYeDxbRKu8qV5aoSogUQ5qVSTSjn7hJ8FQ")), s)
}

func NewRandom(supply ...uint64) *UtxoDB {
	s := defaultSupply
	if len(supply) > 0 {
		s = supply[0]
	}
	var rnd [32]byte
	rand.Read(rnd[:])
	return newUtxodb(ed25519.NewSeed(rnd[:]), s)
}

func (u *UtxoDB) NewKeyPairByIndex(index int) (*ed25519.KeyPair, *ledgerstate.ED25519Address) {
	kp := u.seed.KeyPair(uint64(index))
	return kp, ledgerstate.NewED25519Address(kp.PublicKey)
}

func (u *UtxoDB) genesisInit() {
	// create genesis transaction
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(ledgerstate.NewOutputID(ledgerstate.TransactionID{}, 0)))
	output := ledgerstate.NewSigLockedSingleOutput(defaultSupply, u.GetGenesisAddress())
	outputs := ledgerstate.NewOutputs(output)
	essence := ledgerstate.NewTransactionEssence(essenceVersion, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	signature := ledgerstate.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlock := ledgerstate.NewSignatureUnlockBlock(signature)
	genesisTx := ledgerstate.NewTransaction(essence, ledgerstate.UnlockBlocks{unlockBlock})

	u.genesisTxId = genesisTx.ID()
	u.transactions[u.genesisTxId] = genesisTx
	u.utxo[output.ID()] = output.Clone()
}

// GetGenesisSigScheme return signature scheme used by creator of genesis
func (u *UtxoDB) GetGenesisKeyPair() *ed25519.KeyPair {
	return u.genesisKeyPair
}

// GetGenesisAddress return address of genesis
func (u *UtxoDB) GetGenesisAddress() ledgerstate.Address {
	return u.genesisAddress
}

func (u *UtxoDB) mustRequestFundsTx(target ledgerstate.Address) *ledgerstate.Transaction {
	sourceOutputs := u.GetAddressOutputs(u.GetGenesisAddress())
	if len(sourceOutputs) != 1 {
		panic("number of genesis outputs must be 1")
	}
	reminder, _ := sourceOutputs[0].Balances().Get(ledgerstate.ColorIOTA)
	o1 := ledgerstate.NewSigLockedSingleOutput(RequestFundsAmount, target)
	o2 := ledgerstate.NewSigLockedSingleOutput(reminder-RequestFundsAmount, u.GetGenesisAddress())
	outputs := ledgerstate.NewOutputs(o1, o2)
	inputs := ledgerstate.NewInputs(ledgerstate.NewUTXOInput(sourceOutputs[0].ID()))
	essence := ledgerstate.NewTransactionEssence(0, time.Now(), identity.ID{}, identity.ID{}, inputs, outputs)
	signature := ledgerstate.NewED25519Signature(u.genesisKeyPair.PublicKey, u.genesisKeyPair.PrivateKey.Sign(essence.Bytes()))
	unlockBlocks := []ledgerstate.UnlockBlock{ledgerstate.NewSignatureUnlockBlock(signature)}
	return ledgerstate.NewTransaction(essence, unlockBlocks)
}

// RequestFunds implements faucet: it sends 1337 IOTA tokens from genesis to the given address.
func (u *UtxoDB) RequestFunds(target ledgerstate.Address) (*ledgerstate.Transaction, error) {
	tx := u.mustRequestFundsTx(target)
	return tx, u.AddTransaction(tx)
}
