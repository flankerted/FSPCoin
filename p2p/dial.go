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

package p2p

import (
	"container/heap"
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/p2p/netutil"
	//storage"github.com/contatract/go-contatract/blizzard/storage"
)

// IP address lengths (bytes).
const (
	IPv4length = 4
	//IPv6length = 16

	// Bigger than we need, not too big to worry about overflow
	big = 0xFFFFFF
)

const (
	// This is the amount of time spent waiting in between
	// redialing a certain node.
	dialHistoryExpiration = 30 * time.Second

	// Discovery lookups are throttled and can only run
	// once every few seconds.
	lookupInterval = 4 * time.Second

	// If no peers are found for this amount of time, the initial bootnodes are
	// attempted to be connected.
	fallbackInterval = 20 * time.Second

	// Endpoint resolution is throttled with bounded backoff.
	initialResolveDelay = 60 * time.Second
	maxResolveDelay     = time.Hour
)

// NodeDialer is used to connect to nodes in the network, typically by using
// an underlying net.Dialer but also using net.Pipe in tests
type NodeDialer interface {
	Dial(*discover.Node) (net.Conn, error)
}

// TCPDialer implements the NodeDialer interface by using a net.Dialer to
// create TCP connections to nodes in the network
type TCPDialer struct {
	*net.Dialer
}

// Dial creates a TCP connection to the node
func (t TCPDialer) Dial(dest *discover.Node) (net.Conn, error) {
	addr := &net.TCPAddr{IP: dest.IP, Port: int(dest.TCP)}
	return t.Dialer.Dial("tcp", addr.String())
}

// dialstate schedules dials and discovery lookups.
// it get's a chance to compute new tasks on every iteration
// of the main loop in Server.run.
type dialstate struct {
	maxDynDials   int
	ntab          discoverTable
	netrestrict   *netutil.Netlist
	netpoliceOnly *netutil.Netlist
	policeWorking *bool

	lookupRunning bool
	dialing       map[discover.NodeID]connFlag
	lookupBuf     []*discover.Node // current discovery lookup results
	randomNodes   []*discover.Node // filled from Table
	static        map[discover.NodeID]*dialTask
	hist          *dialHistory

	start     time.Time        // time when the dialer was first used
	bootnodes []*discover.Node // default dials when there are no peers
}

type discoverTable interface {
	Self() *discover.Node
	Close()
	Resolve(target discover.NodeID) *discover.Node
	Lookup(target discover.NodeID) []*discover.Node
	ReadRandomNodes([]*discover.Node) int

	ReadRandomChunkShardingNodes(shardingChunkIdx int, maxAge time.Duration, buf []*discover.Node, olds []*discover.Node) int
	ReadRandomChunkShardingNodesFromDB(shardingChunkIdx int, maxAge time.Duration, buf []*discover.Node, olds []*discover.Node) int
}

// the dial history remembers recent dials.
type dialHistory []pastDial

// pastDial is an entry in the dial history.
type pastDial struct {
	id  discover.NodeID
	exp time.Time
}

type Device interface {
	Start() (err error)

	Stop()

	Self() *discover.Node

	SetupConn(fd net.Conn, flags connFlag, dialDest *discover.Node, cbData interface{}, peerProtocol string, peerNotiFeed *event.Feed) error
}

type task interface {
	Do(*Device)
}

// A dialTask is generated for each node that is dialed. Its
// fields cannot be accessed while the task is running.
type dialTask struct {
	flags        connFlag
	dest         *discover.NodeCB
	lastResolved time.Time
	resolveDelay time.Duration

	customData interface{}

	peerProtocol string
	peerNotiFeed *event.Feed
}

// discoverTask runs discovery table operations.
// Only one discoverTask is active at any time.
// discoverTask.Do performs a random lookup.
type discoverTask struct {
	results []*discover.Node
}

// A waitExpireTask is generated if there are no other tasks
// to keep the loop in Server.run ticking.
type waitExpireTask struct {
	time.Duration
}

func newDialState(static []*discover.Node, bootnodes []*discover.Node, ntab discoverTable, maxdyn int,
	netrestrict, netPoliceOnly *netutil.Netlist, policeIsWorking *bool) *dialstate {
	s := &dialstate{
		maxDynDials:   maxdyn,
		ntab:          ntab,
		netrestrict:   netrestrict,
		netpoliceOnly: netPoliceOnly,
		policeWorking: policeIsWorking,
		static:        make(map[discover.NodeID]*dialTask),
		dialing:       make(map[discover.NodeID]connFlag),
		bootnodes:     make([]*discover.Node, len(bootnodes)),
		randomNodes:   make([]*discover.Node, maxdyn/2),
		hist:          new(dialHistory),
	}
	copy(s.bootnodes, bootnodes)
	for _, n := range static {
		s.addStatic(&discover.NodeCB{Node: n, Cb: nil, PeerProtocol: "", PeerFeed: nil})
	}
	return s
}

func (s *dialstate) addStatic(n *discover.NodeCB) {
	// This overwites the task instead of updating an existing
	// entry, giving users the opportunity to force a resolve operation.
	s.static[n.Node.ID] = &dialTask{flags: staticDialedConn, dest: n}
}

func (s *dialstate) removeStatic(n *discover.Node) {
	// This removes a task so future attempts to connect will not be made.
	delete(s.static, n.ID)
}

func (s *dialstate) newTasks(nRunning int, peers map[discover.NodeID]*Peer, now time.Time) []task {
	if s.start.IsZero() {
		s.start = now
	}

	var newtasks []task
	addDial := func(flag connFlag, n *discover.Node) bool {
		// added by jianghan
		p := peers[n.ID]
		if p != nil {
			if err := s.checkDial(n, peers); err != nil {
				log.Trace("Skipping dial candidate", "id", n.ID, "addr", &net.TCPAddr{IP: n.IP, Port: int(n.TCP)}, "err", err)
				return false
			}
			// 如果已经建立了连接，那么我们只需要再触发一下协议连接流程
			//log.Info("66666666666666", "id", n.ID, "addr", &net.TCPAddr{IP: n.IP, Port: int(n.TCP)})
			select {
			case p.ReStartProtocolChan <- true:
				//log.Info("addDial p.ReStartProtocolChan")
			default:
			}
			return false
		}
		// end add

		if err := s.checkDial(n, peers); err != nil {
			log.Trace("Skipping dial candidate", "id", n.ID, "addr", &net.TCPAddr{IP: n.IP, Port: int(n.TCP)}, "err", err)
			return false
		}
		s.dialing[n.ID] = flag
		newtasks = append(newtasks, &dialTask{flags: flag, dest: &discover.NodeCB{Node: n, Cb: nil, PeerProtocol: "", PeerFeed: nil}})
		return true
	}

	// Compute number of dynamic dials necessary at this point.
	needDynDials := s.maxDynDials
	for _, p := range peers {
		if p.rw.is(dynDialedConn) {
			needDynDials--
		}
	}
	for _, flag := range s.dialing {
		if flag&dynDialedConn != 0 {
			needDynDials--
		}
	}

	// Expire the dial history on every invocation.
	s.hist.expire(now)

	// Create dials for static nodes if they are not connected.
	for id, t := range s.static {
		//log.Info("range", "id", id, "t", t)
		// added by jianghan
		// p := peers[t.dest.Node.ID]
		// if p != nil {
		// 	// 如果已经建立了连接，那么我们只需要再触发一下协议连接流程
		// 	// 在协议连接流程执行前，先将参数重新设置一次，方便协议握手成功
		// 	p.rw.customData = t.dest.Cb
		// 	log.Info("range", "p.rw.customData", p.rw.customData)
		// 	select {
		// 	case p.ReStartProtocolChan <- true:
		// 		log.Info("range s.static p.ReStartProtocolChan")
		// 	default:
		// 	}
		// 	deleteNodeId = append(deleteNodeId, t.dest.Node.ID)
		// 	continue
		// }
		// end add
		err := s.checkDial(t.dest.Node, peers)
		switch err {
		case errNotWhitelisted, errSelf:
			log.Warn("Removing static dial candidate", "id", t.dest.Node.ID, "addr", &net.TCPAddr{IP: t.dest.Node.IP, Port: int(t.dest.Node.TCP)}, "err", err)
			delete(s.static, t.dest.Node.ID)
		case nil:
			s.dialing[id] = t.flags
			// log.Info("for s.static", "id", id, "t", t)
			newtasks = append(newtasks, t)
		}
	}

	// If we don't have any peers whatsoever, try to dial a random bootnode. This
	// scenario is useful for the testnet (and private networks) where the discovery
	// table might be full of mostly bad peers, making it hard to find good ones.
	if len(peers) == 0 && len(s.bootnodes) > 0 && needDynDials > 0 && now.Sub(s.start) > fallbackInterval {
		bootnode := s.bootnodes[0]
		s.bootnodes = append(s.bootnodes[:0], s.bootnodes[1:]...)
		s.bootnodes = append(s.bootnodes, bootnode)

		if addDial(dynDialedConn, bootnode) {
			needDynDials--
		}
	}
	// Use random nodes from the table for half of the necessary
	// dynamic dials.
	randomCandidates := needDynDials / 2
	if randomCandidates > 0 {
		n := s.ntab.ReadRandomNodes(s.randomNodes)
		for i := 0; i < randomCandidates && i < n; i++ {
			if addDial(dynDialedConn, s.randomNodes[i]) {
				needDynDials--
			}
		}
	}
	// Create dynamic dials from random lookup results, removing tried
	// items from the result buffer.
	i := 0
	for ; i < len(s.lookupBuf) && needDynDials > 0; i++ {
		if addDial(dynDialedConn, s.lookupBuf[i]) {
			needDynDials--
		}
	}
	s.lookupBuf = s.lookupBuf[:copy(s.lookupBuf, s.lookupBuf[i:])]
	// Launch a discovery lookup if more candidates are needed.
	if len(s.lookupBuf) < needDynDials && !s.lookupRunning {
		s.lookupRunning = true
		newtasks = append(newtasks, &discoverTask{})
	}

	// Launch a timer to wait for the next node to expire if all
	// candidates have been tried and no task is currently active.
	// This should prevent cases where the dialer logic is not ticked
	// because there are no pending events.
	if nRunning == 0 && len(newtasks) == 0 && s.hist.Len() > 0 {
		t := &waitExpireTask{s.hist.min().exp.Sub(now)}
		newtasks = append(newtasks, t)
	}
	return newtasks
}

func (s *dialstate) newClientTasks(nRunning int, peers map[discover.NodeID]*Peer, now time.Time, remote remoteAddr) []task {
	if s.start.IsZero() {
		s.start = now
	}

	var newtasks []task
	addDial := func(flag connFlag, n *discover.Node) bool {
		// added by jianghan
		p := peers[n.ID]
		if p != nil {
			select {
			case p.ReStartProtocolChan <- true:
				//log.Info("addDial p.ReStartProtocolChan")
			default:
			}
			return false
		}
		// end add

		if err := s.checkDial(n, peers); err != nil {
			log.Trace("Skipping dial candidate", "id", n.ID, "addr", &net.TCPAddr{IP: n.IP, Port: int(n.TCP)}, "err", err)
			return false
		}
		s.dialing[n.ID] = flag
		newtasks = append(newtasks, &dialTask{flags: flag, dest: &discover.NodeCB{Node: n, Cb: nil, PeerProtocol: "", PeerFeed: nil}})
		return true
	}

	// Compute number of dynamic dials necessary at this point.
	needDynDials := s.maxDynDials
	for _, p := range peers {
		if p.rw.is(dynDialedConn) {
			needDynDials--
		}
	}
	for _, flag := range s.dialing {
		if flag&dynDialedConn != 0 {
			needDynDials--
		}
	}

	// Expire the dial history on every invocation.
	s.hist.expire(now)

	// Create dials for static nodes if they are not connected.
	for id, t := range s.static {
		err := s.checkDial(t.dest.Node, peers)
		switch err {
		case errNotWhitelisted, errSelf:
			log.Warn("Removing static dial candidate", "id", t.dest.Node.ID, "addr", &net.TCPAddr{IP: t.dest.Node.IP, Port: int(t.dest.Node.TCP)}, "err", err)
			delete(s.static, t.dest.Node.ID)
		case nil:
			s.dialing[id] = t.flags
			// log.Info("for s.static", "id", id, "t", t)
			newtasks = append(newtasks, t)
		}
	}

	// If we don't have any peers whatsoever, try to dial a random bootnode. This
	// scenario is useful for the testnet (and private networks) where the discovery
	// table might be full of mostly bad peers, making it hard to find good ones.
	if len(peers) == 0 && len(s.bootnodes) > 0 && needDynDials > 0 && now.Sub(s.start) > fallbackInterval {
		bootnode := s.bootnodes[0]
		s.bootnodes = append(s.bootnodes[:0], s.bootnodes[1:]...)
		s.bootnodes = append(s.bootnodes, bootnode)

		if addDial(dynDialedConn, bootnode) {
			needDynDials--
		}
	}
	//// Use random nodes from the table for half of the necessary
	//// dynamic dials.
	//randomCandidates := needDynDials / 2
	//if randomCandidates > 0 {
	//	n := s.ntab.ReadRandomNodes(s.randomNodes)
	//	for i := 0; i < randomCandidates && i < n; i++ {
	//		if addDial(dynDialedConn, s.randomNodes[i]) {
	//			needDynDials--
	//		}
	//	}
	//}

	randomCandidates := needDynDials
	nodeId, _ := discover.BytesID(remote.ID)
	if randomCandidates > 0 {
		for i := 0; i < randomCandidates; i++ {
			n := discover.Node{
				IP:  parseIPv4(remote.IP),
				UDP: remote.Port,
				TCP: remote.Port,
				ID:  nodeId,
			}
			if addDial(dynDialedConn, &n) {
				needDynDials--
			}
		}
	}

	// Create dynamic dials from random lookup results, removing tried
	// items from the result buffer.
	i := 0
	for ; i < len(s.lookupBuf) && needDynDials > 0; i++ {
		if addDial(dynDialedConn, s.lookupBuf[i]) {
			needDynDials--
		}
	}
	s.lookupBuf = s.lookupBuf[:copy(s.lookupBuf, s.lookupBuf[i:])]
	// Launch a discovery lookup if more candidates are needed.
	if len(s.lookupBuf) < needDynDials && !s.lookupRunning {
		s.lookupRunning = true
		newtasks = append(newtasks, &discoverTask{})
	}

	// Launch a timer to wait for the next node to expire if all
	// candidates have been tried and no task is currently active.
	// This should prevent cases where the dialer logic is not ticked
	// because there are no pending events.
	if nRunning == 0 && len(newtasks) == 0 && s.hist.Len() > 0 {
		t := &waitExpireTask{s.hist.min().exp.Sub(now)}
		newtasks = append(newtasks, t)
	}
	return newtasks
}

// Parse IPv4 address (d.d.d.d).
func parseIPv4(s string) net.IP {
	var p [IPv4length]byte
	for i := 0; i < IPv4length; i++ {
		if len(s) == 0 {
			// Missing octets.
			return nil
		}
		if i > 0 {
			if s[0] != '.' {
				return nil
			}
			s = s[1:]
		}
		n, c, ok := dtoi(s)
		if !ok || n > 0xFF {
			return nil
		}
		s = s[c:]
		p[i] = byte(n)
	}
	if len(s) != 0 {
		return nil
	}
	return net.IPv4(p[0], p[1], p[2], p[3])
}

// Decimal to integer.
// Returns number, characters consumed, success.
func dtoi(s string) (n int, i int, ok bool) {
	n = 0
	for i = 0; i < len(s) && '0' <= s[i] && s[i] <= '9'; i++ {
		n = n*10 + int(s[i]-'0')
		if n >= big {
			return big, i, false
		}
	}
	if i == 0 {
		return 0, 0, false
	}
	return n, i, true
}

var (
	errSelf             = errors.New("is self")
	errAlreadyDialing   = errors.New("already dialing")
	errAlreadyConnected = errors.New("already connected")
	errRecentlyDialed   = errors.New("recently dialed")
	errNotWhitelisted   = errors.New("not contained in netrestrict whitelist")
)

func (s *dialstate) checkDial(n *discover.Node, peers map[discover.NodeID]*Peer) error {
	// JiangHan:
	// 凡是已经建立了连接的对端，这里是不予再次连接的
	_, dialing := s.dialing[n.ID]
	switch {
	case dialing:
		return errAlreadyDialing
	case peers[n.ID] != nil:
		return errAlreadyConnected
	case s.ntab != nil && n.ID == s.ntab.Self().ID:
		return errSelf
	case (s.netrestrict != nil && !s.netrestrict.Contains(n.IP)) ||
		(s.policeWorking != nil && !*s.policeWorking && s.netpoliceOnly != nil && s.netpoliceOnly.Contains(n.IP)):
		return errNotWhitelisted
	case s.hist.contains(n.ID):
		return errRecentlyDialed
	}
	return nil
}

func (s *dialstate) taskDone(t task, now time.Time) {
	switch t := t.(type) {
	case *dialTask:
		s.hist.add(t.dest.Node.ID, now.Add(dialHistoryExpiration))
		delete(s.dialing, t.dest.Node.ID)
	case *discoverTask:
		s.lookupRunning = false
		s.lookupBuf = append(s.lookupBuf, t.results...)
	}
}

func (t *dialTask) Do(dev *Device) {
	if t.dest.Node.Incomplete() {
		if !t.resolve(dev) {
			return
		}
	}
	err := t.dial(dev, t.dest.Node, t.dest.Cb, t.dest.PeerProtocol, t.dest.PeerFeed)
	if err != nil {
		log.Trace("Dial error", "task", t, "err", err)
		// Try resolving the ID of static nodes if dialing failed.
		if _, ok := err.(*dialError); ok && t.flags&staticDialedConn != 0 {
			if t.resolve(dev) {
				// 这次不做判断，因为已经resolve过了，再不成功也不做后继处理了
				t.dial(dev, t.dest.Node, t.dest.Cb, t.dest.PeerProtocol, t.dest.PeerFeed)
			}
		}
	}
}

// resolve attempts to find the current endpoint for the destination
// using discovery.
//
// Resolve operations are throttled with backoff to avoid flooding the
// discovery network with useless queries for nodes that don't exist.
// The backoff delay resets when the node is found.
func (t *dialTask) resolve(d *Device) bool {
	switch dev := (*d).(type) {
	case *Server:
		if dev.ntab == nil {
			log.Debug("Can't resolve node", "id", t.dest.Node.ID, "err", "discovery is disabled")
			return false
		}
		if t.resolveDelay == 0 {
			t.resolveDelay = initialResolveDelay
		}
		if time.Since(t.lastResolved) < t.resolveDelay {
			return false
		}
		resolved := dev.ntab.Resolve(t.dest.Node.ID)
		t.lastResolved = time.Now()
		if resolved == nil {
			t.resolveDelay *= 2
			if t.resolveDelay > maxResolveDelay {
				t.resolveDelay = maxResolveDelay
			}
			log.Debug("Resolving node failed", "id", t.dest.Node.ID, "newdelay", t.resolveDelay)
			return false
		}
		// The node was found.
		t.resolveDelay = initialResolveDelay
		t.dest.Node = resolved
		log.Debug("Resolved node", "id", t.dest.Node.ID, "addr", &net.TCPAddr{IP: t.dest.Node.IP, Port: int(t.dest.Node.TCP)})
		return true

	case *Client:
		if dev.ntab == nil {
			log.Debug("Can't resolve node", "id", t.dest.Node.ID, "err", "discovery is disabled")
			return false
		}
		if t.resolveDelay == 0 {
			t.resolveDelay = initialResolveDelay
		}
		if time.Since(t.lastResolved) < t.resolveDelay {
			return false
		}
		resolved := dev.ntab.Resolve(t.dest.Node.ID)
		t.lastResolved = time.Now()
		if resolved == nil {
			t.resolveDelay *= 2
			if t.resolveDelay > maxResolveDelay {
				t.resolveDelay = maxResolveDelay
			}
			log.Debug("Resolving node failed", "id", t.dest.Node.ID, "newdelay", t.resolveDelay)
			return false
		}
		// The node was found.
		t.resolveDelay = initialResolveDelay
		t.dest.Node = resolved
		log.Debug("Resolved node", "id", t.dest.Node.ID, "addr", &net.TCPAddr{IP: t.dest.Node.IP, Port: int(t.dest.Node.TCP)})
		return true
	}
	return false
}

type dialError struct {
	error
}

// JiangHan: 连接 peer 的实际调用过程
// dial performs the actual connection attempt.
func (t *dialTask) dial(d *Device, dest *discover.Node, cbData interface{}, peerProtocol string, peerNotiFeed *event.Feed) error {
	switch dev := (*d).(type) {
	case *Server:
		fd, err := dev.Dialer.Dial(dest)
		if err != nil {
			return &dialError{err}
		}
		mfd := newMeteredConn(fd, false)
		return dev.SetupConn(mfd, t.flags, dest, cbData, peerProtocol, peerNotiFeed)

	case *Client:
		fd, err := dev.Dialer.Dial(dest)
		if err != nil {
			return &dialError{err}
		}
		mfd := newMeteredConn(fd, false)
		return dev.SetupConn(mfd, t.flags, dest, cbData, peerProtocol, peerNotiFeed)
	}

	return errors.New("dial error")
}

func (t *dialTask) String() string {
	return fmt.Sprintf("%v %x %v:%d", t.flags, t.dest.Node.ID[:8], t.dest.Node.IP, t.dest.Node.TCP)
}

func (t *discoverTask) Do(d *Device) {
	// newTasks generates a lookup task whenever dynamic dials are
	// necessary. Lookups need to take some time, otherwise the
	// event loop spins too fast.
	switch dev := (*d).(type) {
	case *Server:
		next := dev.lastLookup.Add(lookupInterval)
		if now := time.Now(); now.Before(next) {
			time.Sleep(next.Sub(now))
		}
		dev.lastLookup = time.Now()
		var target discover.NodeID
		rand.Read(target[:])
		t.results = dev.ntab.Lookup(target)

	case *Client:
		next := dev.lastLookup.Add(lookupInterval)
		if now := time.Now(); now.Before(next) {
			time.Sleep(next.Sub(now))
		}
		dev.lastLookup = time.Now()
		var target discover.NodeID
		rand.Read(target[:])
		t.results = dev.ntab.Lookup(target)
	}
}

func (t *discoverTask) String() string {
	s := "discovery lookup"
	if len(t.results) > 0 {
		s += fmt.Sprintf(" (%d results)", len(t.results))
	}
	return s
}

func (t waitExpireTask) Do(*Device) {
	time.Sleep(t.Duration)
}
func (t waitExpireTask) String() string {
	return fmt.Sprintf("wait for dial hist expire (%v)", t.Duration)
}

// Use only these methods to access or modify dialHistory.
func (h dialHistory) min() pastDial {
	return h[0]
}
func (h *dialHistory) add(id discover.NodeID, exp time.Time) {
	heap.Push(h, pastDial{id, exp})
}
func (h dialHistory) contains(id discover.NodeID) bool {
	for _, v := range h {
		if v.id == id {
			return true
		}
	}
	return false
}
func (h *dialHistory) expire(now time.Time) {
	for h.Len() > 0 && h.min().exp.Before(now) {
		heap.Pop(h)
	}
}

// heap.Interface boilerplate
func (h dialHistory) Len() int           { return len(h) }
func (h dialHistory) Less(i, j int) bool { return h[i].exp.Before(h[j].exp) }
func (h dialHistory) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *dialHistory) Push(x interface{}) {
	*h = append(*h, x.(pastDial))
}
func (h *dialHistory) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
