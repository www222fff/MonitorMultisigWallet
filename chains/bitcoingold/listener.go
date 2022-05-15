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
var BlockRetryInterval = time.Second * 1
var BlockRetryLimit = 5

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

			for txid, utxo := range utxos {
				utxoMap[txid].Refresh = true;

				if _, ok := utxoMap[txid]; ok {
					l.log.Info("existing", "utxo", utxo)
				} else {
					l.log.Info("new added", "utxo", utxo)
					deltaUtxoMap[txid] = utxo
				}
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
	                        err = l.generateDepositEventsForUtxo(v])
		                if err != nil {
					l.log.Error("Failed to generate events for utxo", "tx", v, "err", err)
				}
			}

			//pooling interval
			time.Sleep(BlockRetryInterval)
                        retry = BlockRetryLimit
		}
	}
}

func (l *listener) generateDepositEventsForUtxo(utxo bitcoind.UTXO) error {
        l.log.Debug("Querying block for deposit events", "block", latestBlock)
        query := buildQuery(l.cfg.bridgeContract, utils.Deposit, latestBlock, latestBlock)

        // querying for logs
        logs, err := l.conn.Client().FilterLogs(context.Background(), query)
        if err != nil {
                return fmt.Errorf("unable to Filter Logs: %w", err)
        }

        // read through the log events and handle their deposit event if handler is recognized
        for _, log := range logs {
                var m msg.Message
                destId := msg.ChainId(log.Topics[1].Big().Uint64())
                rId := msg.ResourceIdFromSlice(log.Topics[2].Bytes())
                nonce := msg.Nonce(log.Topics[3].Big().Uint64())

                addr, err := l.bridgeContract.ResourceIDToHandlerAddress(&bind.CallOpts{From: l.conn.Keypair().CommonAddress()}, rId)
                if err != nil {
                        return fmt.Errorf("failed to get handler from resource ID %x", rId)
                }

                m, err = l.handleBtgDepositedEvent(destId, nonce)
                if err != nil {
                        return err
                }

                err = l.router.Send(m)
                if err != nil {
                        l.log.Error("subscription error: failed to route message", "err", err)
                }
        }
}

func (l *listener) handleBtgDepositedEvent(destId msg.ChainId, nonce msg.Nonce) (msg.Message, error) {
        l.log.Info("Handling fungible deposit event", "dest", destId, "nonce", nonce)

        record, err := l.erc20HandlerContract.GetDepositRecord(&bind.CallOpts{From: l.conn.Keypair().CommonAddress()}, uint64(nonce), uint8(destId))
        if err != nil {
                l.log.Error("Error Unpacking ERC20 Deposit Record", "err", err)
                return msg.Message{}, err
        }

        return msg.NewFungibleTransfer(
                l.cfg.id,
                destId,
                nonce,
                record.Amount,
                record.ResourceID,
                record.DestinationRecipientAddress,
        ), nil
}

