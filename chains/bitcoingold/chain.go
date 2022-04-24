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
	"github.com/Phala-Network/chainbridge-utils/blockstore"
	"github.com/Phala-Network/chainbridge-utils/core"
	"github.com/Phala-Network/chainbridge-utils/crypto/sr25519"
	"github.com/Phala-Network/chainbridge-utils/keystore"
	metrics "github.com/Phala-Network/chainbridge-utils/metrics/types"
	"github.com/Phala-Network/chainbridge-utils/msg"
	"github.com/ChainSafe/log15"
        "github.com/www222fff/watchUTXO/go-bitcoind"
)

var _ core.Chain = &Chain{}

type Chain struct {
	cfg      *core.ChainConfig // The config of the chain
	conn     *Connection       // THe chains connection
	listener *listener         // The listener of this chain
	writer   *writer           // The writer of the chain
	stop     chan<- int
}


const (
        SERVER_HOST        = "127.0.0.1"
        SERVER_PORT        = 8332
        RPCUSER            = "danny"
        RPCPASSWD          = "danny_wang"
        USESSL             = false
        WALLET_NAME        = "danny"
        WALLET_PASSPHRASE  = "test"
      )

var watch_addresses = []string{"btg1qmc6uua0jngs9qr38w3pchcvdcrzu878t8p8nwqtj32rtjvjfvnfqywt5pr"}

func findWallet(slice []string, s string) int {
        for index, value := range slice {
                if value == s {
                        return index
                }
        }
        return -1
}


// checkBlockstore queries the blockstore for the latest known block. If the latest block is
// greater than startBlock, then the latest block is returned, otherwise startBlock is.
func checkBlockstore(bs *blockstore.Blockstore, startBlock uint64) (uint64, error) {
	latestBlock, err := bs.TryLoadLatestBlock()
	if err != nil {
		return 0, err
	}

	if latestBlock.Uint64() > startBlock {
		return latestBlock.Uint64(), nil
	} else {
		return startBlock, nil
	}
}

func InitializeChain(cfg *core.ChainConfig, logger log15.Logger, sysErr chan<- error, m *metrics.ChainMetrics) (*Chain, error) {

        bc1, err := bitcoind.New(SERVER_HOST, SERVER_PORT, "", RPCUSER, RPCPASSWD, USESSL)
        if err != nil {
                log.Fatalln(err)
        }

        wallets, err := bc1.ListWallet()
        r := findWallet(wallets, WALLET_NAME)
        if r == -1 {
                err = bc1.LoadWallet(WALLET_NAME, false)
                log.Println(err)
        }

        conn, err := bitcoind.New(SERVER_HOST, SERVER_PORT, WALLET_NAME, RPCUSER, RPCPASSWD, USESSL)
        if err != nil {
                log.Fatalln(err)
        }

        err = bc.WalletPassphrase(WALLET_PASSPHRASE, 100000000)
        log.Println(err)

	stop := make(chan int)

	// Setup listener & writer
	l := NewListener(conn, cfg.Name, cfg.Id, startBlock, logger, bs, stop, sysErr, m)
	w := NewWriter(conn, logger, sysErr, m, ue)
	return &Chain{
		cfg:      cfg,
		conn:     conn,
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
	c.conn.log.Debug("Successfully started chain", "chainId", c.cfg.Id)
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
