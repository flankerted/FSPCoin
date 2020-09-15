package p2p

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/contatract/go-contatract/common"
	"github.com/contatract/go-contatract/common/mclock"
	"github.com/contatract/go-contatract/event"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p/discover"
	"github.com/contatract/go-contatract/p2p/discv5"
	"github.com/contatract/go-contatract/p2p/nat"
	"github.com/contatract/go-contatract/p2p/netutil"
)

var errClientStopped = errors.New("client stopped")

type remoteAddr struct {
	IP   string
	Port uint16
	ID   []byte
}

// p2p.ClientCfg holds node's connection options.
type ClientCfg struct {
	// This field must be set to a valid secp256k1 private key.
	PrivateKey *ecdsa.PrivateKey `toml:"-"`

	// Network ID to use for selecting peers to connect to
	NetworkId uint64

	// remote IP only use for client to connect
	RemoteIP string

	// remote port only use for client to connect
	RemotePort uint16

	// remote ID only use for client to connect
	RemoteID []byte

	// MaxPendingPeers is the maximum number of peers that can be pending in the
	// handshake phase, counted separately for inbound and outbound connections.
	// Zero defaults to preset values.
	MaxPendingPeers int `toml:",omitempty"`

	// DialRatio controls the ratio of inbound to dialed connections.
	// Example: a DialRatio of 2 allows 1/2 of connections to be dialed.
	// Setting DialRatio to zero defaults it to 3.
	DialRatio int `toml:",omitempty"`

	// NoDiscovery can be used to disable the peer discovery mechanism.
	// Disabling is useful for protocol debugging (manual topology).
	NoDiscovery bool

	// DiscoveryV5 specifies whether the the new topic-discovery based V5 discovery
	// protocol should be started or not.
	DiscoveryV5 bool `toml:",omitempty"`

	// Name sets the node name of this client.
	// Use common.MakeName to create a name that follows existing conventions.
	Name string `toml:"-"`

	// BootstrapNodes are used to establish connectivity
	// with the rest of the network.
	BootstrapNodes []*discover.Node

	// BootstrapNodesV5 are used to establish connectivity
	// with the rest of the network using the V5 discovery
	// protocol.
	BootstrapNodesV5 []*discv5.Node `toml:",omitempty"`

	// Static nodes are used as pre-configured connections which are always
	// maintained and re-connected on disconnects.
	StaticNodes []*discover.Node

	// Connectivity can be restricted to certain IP networks.
	// If this option is set to a non-nil value, only hosts which match one of the
	// IP networks contained in the list are considered.
	NetRestrict *netutil.Netlist `toml:",omitempty"`

	// NodeDatabase is the path to the database containing the previously seen
	// live nodes in the network.
	NodeDatabase string `toml:",omitempty"`

	// Protocols should contain the protocols supported
	// by the server. Matching protocols are launched for
	// each peer.
	Protocols []Protocol `toml:"-"`

	// If Dialer is set to a non-nil value, the given Dialer
	// is used to dial outbound peer connections.
	Dialer NodeDialer `toml:"-"`

	// If UDPListenAddr is set to a non-nil address, the client
	// will listen for incoming udp connections.
	//
	// If the address is zero, the operating system will pick a port. The
	// UDPListenAddr field will be updated with the actual address when
	// the client is started.
	UDPListenAddr string

	// If set to a non-nil value, the given NAT port mapper
	// is used to make the listening port available to the
	// Internet.
	NAT nat.Interface `toml:",omitempty"`

	// If EnableMsgEvents is set then the server will emit PeerEvents
	// whenever a message is sent to or received from a peer
	EnableMsgEvents bool

	// Logger is a custom logger to use with the p2p.Server.
	Logger log.Logger `toml:",omitempty"`
}

// Client manages all peer connections.
type Client struct {
	// ClientCfg fields may not be modified while the server is running.
	ClientCfg

	// Hooks for testing. These are useful because we can inhibit
	// the whole protocol stack.
	newTransport func(net.Conn) transport
	newPeerHook  func(*Peer)

	lock    sync.Mutex // protects running
	running bool

	ntab         discoverTable
	ourHandshake *protoHandshake
	lastLookup   time.Time
	DiscV5       *discv5.Network

	// These are for Peers, PeerCount (and nothing else).
	peerOp     chan peerOpFunc
	peerOpDone chan struct{}

	quit          chan struct{}
	addstatic     chan *discover.NodeCB
	removestatic  chan *discover.Node
	posthandshake chan *conn
	addpeer       chan *conn
	delpeer       chan peerDrop
	peerFeed      event.Feed
	log           log.Logger
}

// Stop terminates the server and all active peer connections.
// It blocks until all active connections have been closed.
func (cli *Client) Stop() {
	cli.lock.Lock()
	defer cli.lock.Unlock()
	if !cli.running {
		return
	}
	cli.running = false
	close(cli.quit)
}

// Start starts running the client.
// Clients can not be re-used after stopping.
func (cli *Client) Start() (err error) {
	cli.lock.Lock()
	defer cli.lock.Unlock()
	if cli.running {
		return errors.New("client already running")
	}
	cli.running = true
	cli.log = cli.ClientCfg.Logger
	if cli.log == nil {
		cli.log = log.New()
	}
	cli.log.Info("Starting P2P networking")

	// static fields
	if cli.PrivateKey == nil {
		return fmt.Errorf("Client.PrivateKey must be set to a non-nil key")
	}
	if cli.newTransport == nil {
		cli.newTransport = newRLPX
	}
	if cli.Dialer == nil {
		cli.Dialer = TCPDialer{&net.Dialer{Timeout: defaultDialTimeout}}
	}
	cli.quit = make(chan struct{})
	cli.addpeer = make(chan *conn)
	cli.delpeer = make(chan peerDrop)
	cli.posthandshake = make(chan *conn)
	cli.addstatic = make(chan *discover.NodeCB)
	cli.removestatic = make(chan *discover.Node)
	cli.peerOp = make(chan peerOpFunc)
	cli.peerOpDone = make(chan struct{})

	var (
		conn      *net.UDPConn
		sconn     *sharedUDPConn
		realaddr  *net.UDPAddr
		unhandled chan discover.ReadPacket
	)

	if !cli.NoDiscovery || cli.DiscoveryV5 {
		addr, err := net.ResolveUDPAddr("udp", cli.UDPListenAddr)
		if err != nil {
			return err
		}
		conn, err = net.ListenUDP("udp", addr)
		if err != nil {
			return err
		}
		realaddr = conn.LocalAddr().(*net.UDPAddr)
	}

	if !cli.NoDiscovery && cli.DiscoveryV5 {
		unhandled = make(chan discover.ReadPacket, 100)
		sconn = &sharedUDPConn{conn, unhandled}
	}

	// node table
	if !cli.NoDiscovery {
		cfg := discover.Config{
			PrivateKey:   cli.PrivateKey,
			AnnounceAddr: realaddr,
			NodeDBPath:   cli.NodeDatabase,
			NetRestrict:  cli.NetRestrict,
			Bootnodes:    cli.BootstrapNodes,
			Unhandled:    unhandled,
		}
		ntab, err := discover.ListenUDP(conn, cfg)
		if err != nil {
			return err
		}
		cli.ntab = ntab
	}

	if cli.DiscoveryV5 {
		var (
			ntab *discv5.Network
			err  error
		)
		if sconn != nil {
			ntab, err = discv5.ListenUDP(cli.PrivateKey, sconn, realaddr, "", cli.NetRestrict)
		} else {
			ntab, err = discv5.ListenUDP(cli.PrivateKey, conn, realaddr, "", cli.NetRestrict)
		}
		if err != nil {
			return err
		}
		if err := ntab.SetFallbackNodes(cli.BootstrapNodesV5); err != nil {
			return err
		}
		cli.DiscV5 = ntab
	}

	// jh: 根据配置的maxPeers的一定比率推算需要初始连接的节点数量
	dynPeers := cli.maxDialedConns()
	dialer := newDialState(cli.StaticNodes, cli.BootstrapNodes, cli.ntab, dynPeers, cli.NetRestrict, nil, nil)

	// handshake
	cli.ourHandshake = &protoHandshake{Version: baseProtocolVersion, Name: cli.Name, ID: discover.PubkeyID(&cli.PrivateKey.PublicKey)}
	for _, p := range cli.Protocols {
		cli.ourHandshake.Caps = append(cli.ourHandshake.Caps, p.cap())
	}

	go cli.run(dialer)
	cli.running = true
	return nil
}

func (cli *Client) run(dialstate dialer) {
	var (
		peers        = make(map[discover.NodeID]*Peer)
		inboundCount = 0
		taskdone     = make(chan task, maxActiveDialTasks)
		runningTasks []task
		queuedTasks  []task // tasks that can't run yet
	)

	// removes t from runningTasks
	delTask := func(t task) {
		for i := range runningTasks {
			if runningTasks[i] == t {
				runningTasks = append(runningTasks[:i], runningTasks[i+1:]...)
				break
			}
		}
	}
	// starts until max number of active tasks is satisfied
	startTasks := func(ts []task) (rest []task) {
		i := 0
		for ; len(runningTasks) < maxActiveDialTasks && i < len(ts); i++ {
			t := ts[i]
			cli.log.Trace("New dial task", "task", t)
			var dev Device = cli
			dev = &(*cli)
			go func() { t.Do(&dev); taskdone <- t }()
			runningTasks = append(runningTasks, t)
		}
		return ts[i:]
	}
	scheduleTasks := func() {
		// Start from queue first.
		queuedTasks = append(queuedTasks[:0], startTasks(queuedTasks)...)
		// Query dialer for new tasks and start as many as possible now.
		if len(runningTasks) < maxActiveDialTasks {
			remote := remoteAddr{cli.RemoteIP, cli.RemotePort, cli.RemoteID}
			nt := dialstate.newClientTasks(len(runningTasks)+len(queuedTasks), peers, time.Now(), remote)
			queuedTasks = append(queuedTasks, startTasks(nt)...)
		}
	}

running:
	for {
		scheduleTasks()

		select {
		case <-cli.quit:
			// The server was stopped. Run the cleanup logic.
			break running
		case n := <-cli.addstatic:
			// This channel is used by AddPeer to add to the
			// ephemeral static peer list. Add it to the dialer,
			// it will keep the node connected.
			cli.log.Debug("Adding static node", "node", n.Node)
			//log.Info("run----------------------------------------------", "n", n)
			dialstate.addStatic(n)
		case n := <-cli.removestatic:
			// This channel is used by RemovePeer to send a
			// disconnect request to a peer and begin the
			// stop keeping the node connected
			cli.log.Debug("Removing static node", "node", n)
			dialstate.removeStatic(n)
			if p, ok := peers[n.ID]; ok {
				p.Disconnect(DiscRequested)
			}
		case op := <-cli.peerOp:
			// This channel is used by Peers and PeerCount.
			op(peers)
			cli.peerOpDone <- struct{}{}
		case t := <-taskdone:
			// A task got done. Tell dialstate about it so it
			// can update its state and remove it from the active
			// tasks list.
			cli.log.Trace("Dial task done", "task", t)
			dialstate.taskDone(t, time.Now())
			delTask(t)

		case c := <-cli.posthandshake:
			// A connection has passed the encryption handshake so
			// the remote identity is known (but hasn't been verified yet).
			// TODO: track in-progress inbound node IDs (pre-Peer) to avoid dialing them.
			select {
			case c.cont <- cli.encHandshakeChecks(peers, inboundCount, c):
			case <-cli.quit:
				break running
			}
		case c := <-cli.addpeer:
			// At this point the connection is past the protocol handshake.
			// Its capabilities are known and the remote identity is verified.
			err := cli.protoHandshakeChecks(peers, inboundCount, c)
			if err == nil {
				// The handshakes are done and it passed all checks.
				p := newPeer(c, cli.Protocols)
				// If message events are enabled, pass the peerFeed
				// to the peer
				if cli.EnableMsgEvents {
					p.events = &cli.peerFeed
				}
				name := truncateName(c.name)
				cli.log.Debug("Adding p2p peer", "name", name, "addr", c.fd.RemoteAddr(), "peers", len(peers)+1)
				go cli.runPeer(p)
				peers[c.id] = p
				if p.Inbound() {
					// 外界连接过来的，不是自己  dail 出去的
					inboundCount++
				}
			}
			// The dialer logic relies on the assumption that
			// dial tasks complete after the peer has been added or
			// discarded. Unblock the task last.
			select {
			case c.cont <- err:
			case <-cli.quit:
				break running
			}
		case pd := <-cli.delpeer:
			// A peer disconnected.
			d := common.PrettyDuration(mclock.Now() - pd.created)
			pd.log.Debug("Removing p2p peer", "duration", d, "peers", len(peers)-1, "req", pd.requested, "err", pd.err)
			delete(peers, pd.ID())
			if pd.Inbound() {
				inboundCount--
			}
		}
	}

	log.Info("P2P networking is spinning down")

	// Terminate discovery. If there is a running lookup it will terminate soon.
	if cli.ntab != nil {
		cli.ntab.Close()
	}
	if cli.DiscV5 != nil {
		cli.DiscV5.Close()
	}
	// Disconnect all peers.
	for _, p := range peers {
		p.Disconnect(DiscQuitting)
	}
	// Wait for peers to shut down. Pending connections and tasks are
	// not handled here and will terminate soon-ish because cli.quit
	// is closed.
	for len(peers) > 0 {
		p := <-cli.delpeer
		p.log.Trace("<-delpeer (spindown)", "remainingTasks", len(runningTasks))
		delete(peers, p.ID())
	}
}

func (cli *Client) protoHandshakeChecks(peers map[discover.NodeID]*Peer, inboundCount int, c *conn) error {
	// Drop connections with no matching protocols.
	if len(cli.Protocols) > 0 && countMatchingProtocols(cli.Protocols, c.caps) == 0 {
		return DiscUselessPeer
	}
	// Repeat the encryption handshake checks because the
	// peer set might have changed between the handshakes.
	return cli.encHandshakeChecks(peers, inboundCount, c)
}

func (cli *Client) encHandshakeChecks(peers map[discover.NodeID]*Peer, inboundCount int, c *conn) error {
	switch {
	case !c.is(trustedConn|staticDialedConn) && len(peers) >= 2: //cli.MaxPeers:
		return DiscTooManyPeers
	case !c.is(trustedConn) && c.is(inboundConn) && inboundCount >= 2: //cli.maxInboundConns():
		return DiscTooManyPeers
	case peers[c.id] != nil:
		return DiscAlreadyConnected
	case c.id == cli.Self().ID:
		return DiscSelf
	default:
		return nil
	}
}

func (cli *Client) maxDialedConns() int {
	//if cli.NoDiscovery {
	//	return 0
	//}
	//r := cli.DialRatio
	//if r == 0 {
	//	r = defaultDialRatio
	//}
	//return cli.MaxPeers / r
	return 1
}

// Self returns the local node's endpoint information.
func (cli *Client) Self() *discover.Node {
	cli.lock.Lock()
	defer cli.lock.Unlock()

	if !cli.running {
		return &discover.Node{IP: net.ParseIP("0.0.0.0")}
	}
	//return cli.makeSelf(cli.listener, cli.ntab)
	return cli.makeSelf(nil, cli.ntab)
}

func (cli *Client) makeSelf(listener net.Listener, ntab discoverTable) *discover.Node {
	// If the server's not running, return an empty node.
	// If the node is running but discovery is off, manually assemble the node infos.
	if ntab == nil {
		// Inbound connections disabled, use zero address.
		if listener == nil {
			return &discover.Node{IP: net.ParseIP("0.0.0.0"), ID: discover.PubkeyID(&cli.PrivateKey.PublicKey)}
		}
		// Otherwise inject the listener address too
		addr := listener.Addr().(*net.TCPAddr)
		return &discover.Node{
			ID:  discover.PubkeyID(&cli.PrivateKey.PublicKey),
			IP:  addr.IP,
			TCP: uint16(addr.Port),
		}
	}
	// Otherwise return the discovery node.
	return ntab.Self()
}

// SetupConn runs the handshakes and attempts to add the connection
// as a peer. It returns when the connection has been added as a peer
// or the handshakes have failed.
func (cli *Client) SetupConn(fd net.Conn, flags connFlag, dialDest *discover.Node, cbData interface{}, peerProtocol string, peerNotiFeed *event.Feed) error {
	self := cli.Self()
	if self == nil {
		return errors.New("shutdown")
	}
	c := &conn{
		fd:           fd,
		transport:    cli.newTransport(fd),
		flags:        flags,
		cont:         make(chan error),
		customData:   cbData,
		peerProtocol: peerProtocol,
		peerNotiFeed: peerNotiFeed,
	}
	err := cli.setupConn(c, flags, dialDest)
	if err != nil {
		c.close(err)
		cli.log.Trace("Setting up connection failed", "id", c.id, "err", err)
	}
	return err
}

func (cli *Client) setupConn(c *conn, flags connFlag, dialDest *discover.Node) error {
	// Prevent leftover pending conns from entering the handshake.
	cli.lock.Lock()
	running := cli.running
	cli.lock.Unlock()
	if !running {
		return errServerStopped
	}
	// Run the encryption handshake.
	var err error
	if c.id, err = c.doEncHandshake(cli.PrivateKey, dialDest); err != nil {
		cli.log.Trace("Failed RLPx handshake", "addr", c.fd.RemoteAddr(), "conn", c.flags, "err", err)
		return err
	}
	clog := cli.log.New("id", c.id, "addr", c.fd.RemoteAddr(), "conn", c.flags)
	// For dialed connections, check that the remote public key matches.
	if dialDest != nil && c.id != dialDest.ID {
		clog.Trace("Dialed identity mismatch", "want", c, dialDest.ID)
		return DiscUnexpectedIdentity
	}
	err = cli.checkpoint(c, cli.posthandshake)
	if err != nil {
		clog.Trace("Rejected peer before protocol handshake", "err", err)
		return err
	}
	// Run the protocol handshake
	phs, err := c.doProtoHandshake(cli.ourHandshake)
	if err != nil {
		clog.Trace("Failed proto handshake", "err", err)
		return err
	}
	if phs.ID != c.id {
		clog.Trace("Wrong devp2p handshake identity", "err", phs.ID)
		return DiscUnexpectedIdentity
	}
	c.caps, c.name = phs.Caps, phs.Name
	err = cli.checkpoint(c, cli.addpeer)
	if err != nil {
		clog.Trace("Rejected peer", "err", err)
		return err
	}
	// If the checks completed successfully, runPeer has now been
	// launched by run.
	clog.Trace("connection set up", "inbound", dialDest == nil)
	return nil
}

// checkpoint sends the conn to run, which performs the
// post-handshake checks for the stage (posthandshake, addpeer).
func (cli *Client) checkpoint(c *conn, stage chan<- *conn) error {
	select {
	case stage <- c:
	case <-cli.quit:
		return errServerStopped
	}
	select {
	case err := <-c.cont:
		return err
	case <-cli.quit:
		return errServerStopped
	}
}

// runPeer runs in its own goroutine for each peer.
// it waits until the Peer logic returns and removes
// the peer.
func (cli *Client) runPeer(p *Peer) {
	if cli.newPeerHook != nil {
		cli.newPeerHook(p)
	}

	// broadcast peer add
	cli.peerFeed.Send(&PeerEvent{
		Type: PeerEventTypeAdd,
		Peer: p.ID(),
	})

	// run the protocol
	remoteRequested, _, err := p.run()

	// broadcast peer drop
	cli.peerFeed.Send(&PeerEvent{
		Type:  PeerEventTypeDrop,
		Peer:  p.ID(),
		Error: err.Error(),
	})

	// Note: run waits for existing peers to be sent on cli.delpeer
	// before returning, so this send should not select on cli.quit.
	cli.delpeer <- peerDrop{p, err, remoteRequested}
}
