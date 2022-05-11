// Copyright 2020 ChainSafe Systems
// SPDX-License-Identifier: LGPL-3.0-only

package bitcoingold

import (
	"errors"
	"fmt"
	"time"

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
	subscriptions map[eventName]eventHandler // Handlers for specific events
	router        chains.Router
	log           log15.Logger
	stop          <-chan int
	sysErr        chan<- error
	latestBlock   metrics.LatestBlock
	metrics       *metrics.ChainMetrics
}

// Frequency of polling for a new block
var BlockRetryInterval = time.Second * 5

func NewListener(conn *bitcoind.Bitcoind, name string, from string, id msg.ChainId, log log15.Logger, stop <-chan int, sysErr chan<- error, m *metrics.ChainMetrics) *listener {
	return &listener{
		name:          name,
                watchAddr:     []string{from},
		chainId:       id,
		conn:          conn,
		subscriptions: make(map[eventName]eventHandler),
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
	for _, sub := range Subscriptions {
		err := l.registerEventHandler(sub.name, sub.handler)
		if err != nil {
			return err
		}
	}

	go func() {
		err := l.pollBlocks()
		if err != nil {
			l.log.Error("Polling blocks failed", "err", err)
		}
	}()

	return nil
}

// registerEventHandler enables a handler for a given event. This cannot be used after Start is called.
func (l *listener) registerEventHandler(name eventName, handler eventHandler) error {
	if l.subscriptions[name] != nil {
		return fmt.Errorf("event %s already registered", name)
	}
	l.subscriptions[name] = handler
	return nil
}

// pollBlocks will poll for the latest block and proceed to parse the associated events as it sees new blocks.
// Polling begins at the block defined in `l.startBlock`. Failed attempts to fetch the latest block or parse
// a block will be retried up to BlockRetryLimit times before returning with an error.
func (l *listener) pollBlocks() error {

        utxoMap := make(map[string]bitcoind.UTXO)

        for {
		select {
		case <-l.stop:
			return errors.New("terminated")
		default:
			//list all utxo of watched multisig address
			utxos, err := l.conn.ListUnspent(1, 999999, l.watchAddr)
			if err != nil {
				l.log.Error("Listunspent failed", "err", err)
			}

			//filter delta utxo
			deltaUtxoMap := make(map[string]bitcoind.UTXO)
			for _, utxo := range utxos {
				_, ok := utxoMap[utxo.TxID]
				if (ok) {
					l.log.Info("existing ", "utxo", utxo)
				} else {
					l.log.Info("found new ", "utxo", utxo)
					utxoMap[utxo.TxID] = utxo
					deltaUtxoMap[utxo.TxID] = utxo
				}
			}

			//handle new utxo, send deposit event TBD?
			for txid := range deltaUtxoMap {
                                l.log.Info("send deposit event", "txid", txid);
			}

			time.Sleep(BlockRetryInterval)
		}
	}
}

// submitMessage inserts the chainId into the msg and sends it to the router
func (l *listener) submitMessage(m msg.Message, err error) {
	if err != nil {
		log15.Error("Critical error processing event", "err", err)
		return
	}
	m.Source = l.chainId
	err = l.router.Send(m)
	if err != nil {
		log15.Error("failed to process event", "err", err)
	}
}
