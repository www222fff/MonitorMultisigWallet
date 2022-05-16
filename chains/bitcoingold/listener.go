// Copyright 2020 ChainSafe Systems
// SPDX-License-Identifier: LGPL-3.0-only

package bitcoingold

import (
	"errors"
	"time"
	"math/big"
	"github.com/ChainSafe/log15"
	"github.com/Phala-Network/ChainBridge/chains"
	metrics "github.com/Phala-Network/chainbridge-utils/metrics/types"
	"github.com/Phala-Network/chainbridge-utils/msg"
        "github.com/www222fff/watchUTXO/go-bitcoind"
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
var BlockRetryInterval = time.Second * 1
var BlockRetryLimit = 5
var ErrFatalPolling = errors.New("listener UTXO polling failed")

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
	go func() {
		err := l.poolUtxo()
		if err != nil {
			l.log.Error("Polling blocks failed", "err", err)
		}
	}()

	return nil
}

// poolUtxo will poll for the latest block and proceed to parse the associated events as it sees new blocks.
// Polling begins at the block defined in `l.startBlock`. Failed attempts to fetch the latest block or parse
// a block will be retried up to BlockRetryLimit times before returning with an error.
func (l *listener) poolUtxo() error {
	l.log.Info("Polling UTXO...")
	var retry = BlockRetryLimit
        utxoMap := make(map[string]bitcoind.UTXO)

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
			utxos, err := l.conn.ListUnspent(1, 999999, l.watchAddr)
			if err != nil {
				l.log.Error("Listunspent failed", "err", err)
                                retry--
                                time.Sleep(BlockRetryInterval)
                                continue
			}

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
	                        err = l.triggerDepositEvent(v)
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

func (l *listener) triggerDepositEvent(utxo bitcoind.UTXO) error {
        l.log.Debug("Construct deposit events", "utxo", utxo)

	srcId := msg.ChainId(l.chainId)
	destId := msg.ChainId(1)
	nonce := msg.Nonce(123)
        amount := big.NewInt(int64(utxo.Amount))
	rId := msg.ResourceIdFromSlice([]byte(utxo.TxID))
	recipientAddr := []byte("Btg/From/" + utxo.Address)

        m := msg.NewFungibleTransfer(srcId, destId, nonce, amount, rId, recipientAddr)
        l.log.Info("Construct deposit message", "msg", m)
        err := l.router.Send(m)
	if err != nil {
                l.log.Error("subscription error: failed to route message", "err", err)
		return err
	}
	return nil
}
