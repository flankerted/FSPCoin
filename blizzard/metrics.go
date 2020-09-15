// Copyright 2018 The go-contatract Authors
// This file is part of the go-contatract library.
//
// The go-contatract library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-contatract library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-contatract library. If not, see <http://www.gnu.org/licenses/>.

// Contains the meters and timers used by the networking layer.

package blizzard

import (
	"net"

	"github.com/contatract/go-contatract/metrics"
	"github.com/contatract/go-contatract/p2p"
)

var (
	ingressConnectMeter = metrics.NewMeter("blizzard/InboundConnects")
	ingressTrafficMeter = metrics.NewMeter("blizzard/InboundTraffic")
	egressConnectMeter  = metrics.NewMeter("blizzard/OutboundConnects")
	egressTrafficMeter  = metrics.NewMeter("blizzard/OutboundTraffic")
)

// meteredConn is a wrapper around a network TCP connection that meters both the
// inbound and outbound network traffic.
type meteredConn struct {
	*net.TCPConn // Network connection to wrap with metering
}

// newMeteredConn creates a new metered connection, also bumping the ingress or
// egress connection meter. If the metrics system is disabled, this function
// returns the original object.
func newMeteredConn(conn net.Conn, ingress bool) net.Conn {
	// Short circuit if metrics are disabled
	if !metrics.Enabled {
		return conn
	}
	// Otherwise bump the connection counters and wrap the connection
	if ingress {
		ingressConnectMeter.Mark(1)
	} else {
		egressConnectMeter.Mark(1)
	}
	return &meteredConn{conn.(*net.TCPConn)}
}

// Read delegates a network read to the underlying connection, bumping the ingress
// traffic meter along the way.
func (c *meteredConn) Read(b []byte) (n int, err error) {
	n, err = c.TCPConn.Read(b)
	ingressTrafficMeter.Mark(int64(n))
	return
}

// Write delegates a network write to the underlying connection, bumping the
// egress traffic meter along the way.
func (c *meteredConn) Write(b []byte) (n int, err error) {
	n, err = c.TCPConn.Write(b)
	egressTrafficMeter.Mark(int64(n))
	return
}

var (
	propTxnInPacketsMeter     = metrics.NewMeter("blizzard/prop/txns/in/packets")
	propTxnInTrafficMeter     = metrics.NewMeter("blizzard/prop/txns/in/traffic")
	propTxnOutPacketsMeter    = metrics.NewMeter("blizzard/prop/txns/out/packets")
	propTxnOutTrafficMeter    = metrics.NewMeter("blizzard/prop/txns/out/traffic")
	propHashInPacketsMeter    = metrics.NewMeter("blizzard/prop/hashes/in/packets")
	propHashInTrafficMeter    = metrics.NewMeter("blizzard/prop/hashes/in/traffic")
	propHashOutPacketsMeter   = metrics.NewMeter("blizzard/prop/hashes/out/packets")
	propHashOutTrafficMeter   = metrics.NewMeter("blizzard/prop/hashes/out/traffic")
	propBlockInPacketsMeter   = metrics.NewMeter("blizzard/prop/blocks/in/packets")
	propBlockInTrafficMeter   = metrics.NewMeter("blizzard/prop/blocks/in/traffic")
	propBlockOutPacketsMeter  = metrics.NewMeter("blizzard/prop/blocks/out/packets")
	propBlockOutTrafficMeter  = metrics.NewMeter("blizzard/prop/blocks/out/traffic")
	reqHeaderInPacketsMeter   = metrics.NewMeter("blizzard/req/headers/in/packets")
	reqHeaderInTrafficMeter   = metrics.NewMeter("blizzard/req/headers/in/traffic")
	reqHeaderOutPacketsMeter  = metrics.NewMeter("blizzard/req/headers/out/packets")
	reqHeaderOutTrafficMeter  = metrics.NewMeter("blizzard/req/headers/out/traffic")
	reqBodyInPacketsMeter     = metrics.NewMeter("blizzard/req/bodies/in/packets")
	reqBodyInTrafficMeter     = metrics.NewMeter("blizzard/req/bodies/in/traffic")
	reqBodyOutPacketsMeter    = metrics.NewMeter("blizzard/req/bodies/out/packets")
	reqBodyOutTrafficMeter    = metrics.NewMeter("blizzard/req/bodies/out/traffic")
	reqStateInPacketsMeter    = metrics.NewMeter("blizzard/req/states/in/packets")
	reqStateInTrafficMeter    = metrics.NewMeter("blizzard/req/states/in/traffic")
	reqStateOutPacketsMeter   = metrics.NewMeter("blizzard/req/states/out/packets")
	reqStateOutTrafficMeter   = metrics.NewMeter("blizzard/req/states/out/traffic")
	reqReceiptInPacketsMeter  = metrics.NewMeter("blizzard/req/receipts/in/packets")
	reqReceiptInTrafficMeter  = metrics.NewMeter("blizzard/req/receipts/in/traffic")
	reqReceiptOutPacketsMeter = metrics.NewMeter("blizzard/req/receipts/out/packets")
	reqReceiptOutTrafficMeter = metrics.NewMeter("blizzard/req/receipts/out/traffic")
	miscInPacketsMeter        = metrics.NewMeter("blizzard/misc/in/packets")
	miscInTrafficMeter        = metrics.NewMeter("blizzard/misc/in/traffic")
	miscOutPacketsMeter       = metrics.NewMeter("blizzard/misc/out/packets")
	miscOutTrafficMeter       = metrics.NewMeter("blizzard/misc/out/traffic")
)

// meteredMsgReadWriter is a wrapper around a p2p.MsgReadWriter, capable of
// accumulating the above defined metrics based on the data stream contents.
type meteredMsgReadWriter struct {
	p2p.MsgReadWriter     // Wrapped message stream to meter
	version           int // Protocol version to select correct meters
}

// newMeteredMsgWriter wraps a p2p MsgReadWriter with metering support. If the
// metrics system is disabled, this function returns the original object.
func newMeteredMsgWriter(rw p2p.MsgReadWriter) p2p.MsgReadWriter {
	if !metrics.Enabled {
		return rw
	}
	return &meteredMsgReadWriter{MsgReadWriter: rw}
}

// Init sets the protocol version used by the stream to know which meters to
// increment in case of overlapping message ids between protocol versions.
func (rw *meteredMsgReadWriter) Init(version int) {
	rw.version = version
}

func (rw *meteredMsgReadWriter) ReadMsg() (p2p.Msg, error) {
	// Read the message and short circuit in case of an error
	msg, err := rw.MsgReadWriter.ReadMsg()
	if err != nil {
		return msg, err
	}
	// Account for the data traffic
	packets, traffic := miscInPacketsMeter, miscInTrafficMeter
	/*
	switch {
	case msg.Code == BlockHeadersMsg:
		packets, traffic = reqHeaderInPacketsMeter, reqHeaderInTrafficMeter
	case msg.Code == BlockBodiesMsg:
		packets, traffic = reqBodyInPacketsMeter, reqBodyInTrafficMeter

	case rw.version >= elephantv2 && msg.Code == NodeDataMsg:
		packets, traffic = reqStateInPacketsMeter, reqStateInTrafficMeter
	case rw.version >= elephantv2 && msg.Code == ReceiptsMsg:
		packets, traffic = reqReceiptInPacketsMeter, reqReceiptInTrafficMeter

	case msg.Code == NewBlockHashesMsg:
		packets, traffic = propHashInPacketsMeter, propHashInTrafficMeter
	case msg.Code == NewBlockMsg:
		packets, traffic = propBlockInPacketsMeter, propBlockInTrafficMeter
	case msg.Code == TxMsg:
		packets, traffic = propTxnInPacketsMeter, propTxnInTrafficMeter
	}
	*/
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	return msg, err
}

func (rw *meteredMsgReadWriter) WriteMsg(msg p2p.Msg) error {
	// Account for the data traffic
	packets, traffic := miscOutPacketsMeter, miscOutTrafficMeter
	/*
	switch {
	case msg.Code == BlockHeadersMsg:
		packets, traffic = reqHeaderOutPacketsMeter, reqHeaderOutTrafficMeter
	case msg.Code == BlockBodiesMsg:
		packets, traffic = reqBodyOutPacketsMeter, reqBodyOutTrafficMeter

	case rw.version >= elephantv2 && msg.Code == NodeDataMsg:
		packets, traffic = reqStateOutPacketsMeter, reqStateOutTrafficMeter
	case rw.version >= elephantv2 && msg.Code == ReceiptsMsg:
		packets, traffic = reqReceiptOutPacketsMeter, reqReceiptOutTrafficMeter

	case msg.Code == NewBlockHashesMsg:
		packets, traffic = propHashOutPacketsMeter, propHashOutTrafficMeter
	case msg.Code == NewBlockMsg:
		packets, traffic = propBlockOutPacketsMeter, propBlockOutTrafficMeter
	case msg.Code == TxMsg:
		packets, traffic = propTxnOutPacketsMeter, propTxnOutTrafficMeter
	}
	*/
	packets.Mark(1)
	traffic.Mark(int64(msg.Size))

	// Send the packet to the p2p layer
	return rw.MsgReadWriter.WriteMsg(msg)
}