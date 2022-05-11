// Copyright 2022 ChainSafe Systems

/*
The bitcoingold package contains the logic for interacting with bitcoingold chains.
The current supported transfer types are Fungible, Nonfungible, and generic.

There are 3 major components: the connection, the listener, and the writer.

Connection

The connection contains the bitcoingold RPC client and can be accessed by both the writer and listener.

Listener

The substrate listener polls utxos and parses the associated events for the three transfer types. It then forwards these into the router.

Writer

As the writer receives messages from the router, it constructs proposals. If a proposal is still active, the writer will attempt to vote on it. Resource IDs are resolved to method name on-chain, which are then used in the proposals when constructing the resulting Call struct.

*/
package bitcoingold

import (
	"github.com/Phala-Network/chainbridge-utils/core"
	metrics "github.com/Phala-Network/chainbridge-utils/metrics/types"
	"github.com/Phala-Network/chainbridge-utils/msg"
	"github.com/ChainSafe/log15"
        "github.com/www222fff/watchUTXO/go-bitcoind"
)

var _ core.Chain = &Chain{}

type Chain struct {
	cfg      *core.ChainConfig // The config of the chain
	conn     *bitcoind.Bitcoind       // The chains connection
	listener *listener         // The listener of this chain
	writer   *writer           // The writer of the chain
	stop     chan<- int
}


const (
        RPCUSER            = "user"
        RPCPASSWD          = "passwd"
        WALLET_NAME        = "danny"
        WALLET_PASSPHRASE  = "test"
      )


func findWallet(slice []string, s string) int {
        for index, value := range slice {
                if value == s {
                        return index
                }
        }
        return -1
}


func InitializeChain(cfg *core.ChainConfig, logger log15.Logger, sysErr chan<- error, m *metrics.ChainMetrics) (*Chain, error) {

        conn_chain, err := bitcoind.New(cfg.Endpoint, "", RPCUSER, RPCPASSWD, false)
        if err != nil {
                return nil, err
        }

        wallets, err := conn_chain.ListWallet()
        r := findWallet(wallets, WALLET_NAME)
        if r == -1 {
                err = conn_chain.LoadWallet(WALLET_NAME, false)
                if err != nil {
			return nil, err
		}
        }

        conn_wallet, err := bitcoind.New(cfg.Endpoint, WALLET_NAME, RPCUSER, RPCPASSWD, false)
        if err != nil {
		return nil, err
        }

        err = conn_wallet.WalletPassphrase(WALLET_PASSPHRASE, 100000000)
        if err != nil {
		return nil, err
        }

	stop := make(chan int)

	// Setup listener & writer
	l := NewListener(conn_wallet, cfg.Name, cfg.From, cfg.Id, logger, stop, sysErr, m)
	w := NewWriter(conn_wallet, logger, sysErr, m, false)
	return &Chain{
		cfg:      cfg,
		conn:     conn_wallet,
		listener: l,
		writer:   w,
		stop:     stop,
	}, nil
}

func (c *Chain) Start() error {
	err := c.listener.start()
	if err != nil {
		return err
	}
	log15.Debug("Successfully started chain", "chainId", c.cfg.Id)
	return nil
}

func (c *Chain) SetRouter(r *core.Router) {
	r.Listen(c.cfg.Id, c.writer)
	c.listener.setRouter(r)
}

func (c *Chain) LatestBlock() metrics.LatestBlock {
	return c.listener.latestBlock
}

func (c *Chain) Id() msg.ChainId {
	return c.cfg.Id
}

func (c *Chain) Name() string {
	return c.cfg.Name
}

func (c *Chain) Stop() {
	close(c.stop)
}
