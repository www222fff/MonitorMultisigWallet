// Copyright 2020 ChainSafe Systems
// SPDX-License-Identifier: LGPL-3.0-only

package bitcoingold

import (
	"errors"
	"time"
	"strconv"
	"math/big"
	"github.com/ChainSafe/log15"
	"github.com/ChainSafe/ChainBridge/chains"
	metrics "github.com/ChainSafe/chainbridge-utils/metrics/types"
	"github.com/ChainSafe/chainbridge-utils/msg"
        "github.com/www222fff/watchUTXO/go-bitcoind"
	"github.com/ethereum/go-ethereum/common/hexutil"
	utils "github.com/ChainSafe/ChainBridge/shared/substrate"
	"github.com/ChainSafe/chainbridge-utils/keystore"
	"github.com/centrifuge/go-substrate-rpc-client/types"
)

type listener struct {
	name          string
        watchAddr     []string
	chainId       msg.ChainId
	conn          *bitcoind.Bitcoind
	router        chains.Router
	log           log15.Logger
	stop          <-chan int
	sysErr        chan<- error
	latestBlock   metrics.LatestBlock
	metrics       *metrics.ChainMetrics
}

// Frequency of polling for a new block
var BlockRetryInterval = time.Second * 5
var BlockRetryLimit = 5
var ErrFatalPolling = errors.New("listener UTXO polling failed")
var substrateChainId msg.ChainId = 1
var bitcoingoldChain msg.ChainId = 2
var resourceId [32]byte
var AliceKey = keystore.TestKeyRing.SubstrateKeys[keystore.AliceKey].AsKeyringPair()

func NewListener(conn *bitcoind.Bitcoind, name string, from string, id msg.ChainId, log log15.Logger, stop <-chan int, sysErr chan<- error, m *metrics.ChainMetrics) *listener {
	return &listener{
		name:          name,
                watchAddr:     []string{from},
		chainId:       id,
		conn:          conn,
		log:           log,
		stop:          stop,
		sysErr:        sysErr,
		latestBlock:   metrics.LatestBlock{LastUpdated: time.Now()},
		metrics:       m,
	}
}

func (l *listener) setRouter(r chains.Router) {
	l.router = r
}

// start creates the initial subscription for all events
func (l *listener) start() error {

	err := l.initSubstrateChain()
	if err != nil {
		return err
	}

	go func() {
		err := l.poolUtxo()
		if err != nil {
			l.log.Error("Polling blocks failed", "err", err)
		}
	}()

	return nil
}

//danny for test
func (l *listener) initSubstrateChain() error {

	var AliceKey = keystore.TestKeyRing.SubstrateKeys[keystore.AliceKey].AsKeyringPair()

	var relayers = []types.AccountID{
		types.NewAccountID(AliceKey.PublicKey),
	}

	var resources = map[msg.ResourceId]utils.Method{
		// These are taken from the Polkadot JS UI (Chain State -> Constants)
		msg.ResourceIdFromSlice(hexutil.MustDecode("0x000000000000000000000000000000c76ebe4a02bbc34786d860b355f5a5ce00")): utils.ExampleTransferMethod,
	}

	const relayerThreshold = 1
	const TestEndpoint = "ws://127.0.0.1:9944"

        client, err := utils.CreateClient(AliceKey, TestEndpoint)
        if err != nil {
                return err
        }
        err = utils.InitializeChain(client, relayers, []msg.ChainId{bitcoingoldChain}, resources, relayerThreshold)
        if err != nil {
                return err
        }
        err = utils.QueryConst(client, "Example", "NativeTokenId", &resourceId)
        if err != nil {
                return err
        }

	return nil
}

// poolUtxo will poll for the latest block and proceed to parse the associated events as it sees new blocks.
// Polling begins at the block defined in `l.startBlock`. Failed attempts to fetch the latest block or parse
// a block will be retried up to BlockRetryLimit times before returning with an error.
func (l *listener) poolUtxo() error {
	l.log.Info("Polling UTXO...")
	var retry = BlockRetryLimit
        utxoMap := make(map[string]bitcoind.UTXO)
	var nonce = 0

        for {
		select {
		case <-l.stop:
			return errors.New("terminated")
		default:
                        // No more retries, goto next block
                        if retry == 0 {
                                l.log.Error("Polling failed, retries exceeded")
                                l.sysErr <- ErrFatalPolling
                                return nil
                        }

			//list all utxo of watched multisig address
			/*utxos, err := l.conn.ListUnspent(1, 999999, l.watchAddr)
			if err != nil {
				l.log.Error("Listunspent failed", "err", err)
                                retry--
                                time.Sleep(BlockRetryInterval)
                                continue
			}*/

			//danny for test
			var utxos []bitcoind.UTXO
			var utxo bitcoind.UTXO
			nonce ++
			utxo.TxID = "f35103085b7145e569eb8053365c662cb7b9b7fd6009e37cafbb684bd89b638b" + strconv.Itoa(nonce)
			utxo.Amount = 1
			utxo.Address = "btg1qmc6uua0jngs9qr38w3pchcvdcrzu878t8p8nwqtj32rtjvjfvnfqywt5pr"
			utxos = append(utxos, utxo)

			//filter delta utxo
			deltaUtxoMap := make(map[string]bitcoind.UTXO)

			for _, utxo := range utxos {
				if _, ok := utxoMap[utxo.TxID]; ok {
					l.log.Info("existing", "utxo", utxo)
				} else {
					l.log.Info("new added", "utxo", utxo)
					deltaUtxoMap[utxo.TxID] = utxo
				}
				utxo.Refresh = true
				utxoMap[utxo.TxID] = utxo
			}

			//update utxoMap based on the latest utxos
			for k, v := range utxoMap {
				if v.Refresh {
					v.Refresh = false
				} else {
					delete(utxoMap, k)
				}
			}

			//handle new utxo, send deposit event TBD?
			for k, v := range deltaUtxoMap {
                                l.log.Info("send deposit event", "txid", k);
				// Parse out events
	                        err := l.triggerDepositEvent(v, nonce)
		                if err != nil {
					l.log.Error("Failed to trigger events for utxo", "tx", v, "err", err)
				}
			}

			//pooling interval
			time.Sleep(BlockRetryInterval)
                        retry = BlockRetryLimit
		}
	}
}

func (l *listener) triggerDepositEvent(utxo bitcoind.UTXO, nonce int) error {
        l.log.Debug("Construct deposit events", "utxo", utxo)

	srcId := msg.ChainId(l.chainId)
	destId := msg.ChainId(substrateChainId)
	depositNonce := msg.Nonce(nonce)
        amount := big.NewInt(utxo.Amount)
	//recipient := []byte("Btg/FromAddress/" + utxo.Address)
	recipient := AliceKey.PublicKey

        m := msg.NewFungibleTransfer(srcId, destId, depositNonce, amount, resourceId, recipient)
        l.log.Info("Construct deposit message", "msg", m)
        err := l.router.Send(m)
	if err != nil {
                l.log.Error("subscription error: failed to route message", "err", err)
		return err
	}
	return nil
}
