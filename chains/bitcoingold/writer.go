// Copyright 2020 ChainSafe Systems
// SPDX-License-Identifier: LGPL-3.0-only

package bitcoingold

import (
	"github.com/Phala-Network/chainbridge-utils/core"
	metrics "github.com/Phala-Network/chainbridge-utils/metrics/types"
	"github.com/Phala-Network/chainbridge-utils/msg"
	"github.com/ChainSafe/log15"
        "github.com/www222fff/watchUTXO/go-bitcoind"
)

var _ core.Writer = &writer{}

type writer struct {
	conn       *bitcoind.Bitcoind
	log        log15.Logger
	sysErr     chan<- error
	metrics    *metrics.ChainMetrics
	extendCall bool // Extend extrinsic calls to substrate with ResourceID.Used for backward compatibility with example pallet.
}

func NewWriter(conn *bitcoind.Bitcoind, log log15.Logger, sysErr chan<- error, m *metrics.ChainMetrics, extendCall bool) *writer {
	return &writer{
		conn:       conn,
		log:        log,
		sysErr:     sysErr,
		metrics:    m,
		extendCall: extendCall,
	}
}

func (w *writer) ResolveMessage(m msg.Message) bool {

	//TBD
	return true
}
