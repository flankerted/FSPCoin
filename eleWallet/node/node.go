package node

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/contatract/go-contatract/accounts"
	"github.com/contatract/go-contatract/ftransfer"
	"github.com/contatract/go-contatract/log"
	"github.com/contatract/go-contatract/p2p"
	"github.com/prometheus/prometheus/util/flock"
)

// Node is a container on which services can be registered.
type Node struct {
	//eventmux *event.TypeMux // Event multiplexer used between the services of a stack
	config *NodeConfig
	accman *accounts.Manager

	clientConfig p2p.ClientCfg
	client       *p2p.Client // Currently running P2P networking layer

	instanceDirLock flock.Releaser // prevents concurrent use of instance directory

	serviceFuncs []FTransServiceConstructor     // Service constructors (in dependency order)
	services     map[reflect.Type]FTransService // Currently running services

	stop chan struct{} // Channel to wait for termination notifications
	lock sync.RWMutex

	FTransAPI *ftransfer.PrivateFTransAPI
	Peer      *p2p.Peer

	log log.Logger
}

// New creates a new P2P node, ready for protocol registration.
func New(conf *NodeConfig) (*Node, error) {
	// Copy config and resolve the datadir so future changes to the current
	// working directory don't affect the node.
	confCopy := *conf
	conf = &confCopy
	if conf.DataDir != "" {
		absdatadir, err := filepath.Abs(conf.DataDir)
		if err != nil {
			return nil, err
		}
		conf.DataDir = absdatadir
	}
	// Ensure that the AccountManager method works before the node has started.
	// We rely on this in cmd/geth.
	am, err := MakeAccountManager(conf)
	if err != nil {
		return nil, err
	}
	if conf.Logger == nil {
		conf.Logger = log.New()
	}
	// Ensure that the instance name doesn't cause weird conflicts with
	// other files in the data directory.
	if strings.ContainsAny(conf.Name, `/\`) {
		return nil, errors.New(`Config.Name must not contain '/' or '\'`)
	}
	if conf.Name == datadirDefaultKeyStore {
		return nil, errors.New(`Config.Name cannot be "` + datadirDefaultKeyStore + `"`)
	}
	if strings.HasSuffix(conf.Name, ".ipc") {
		return nil, errors.New(`Config.Name cannot end in ".ipc"`)
	}

	// Note: any interaction with Config that would create/touch files
	// in the data directory or instance directory is delayed until Start.
	return &Node{
		accman:       am,
		config:       conf,
		serviceFuncs: []FTransServiceConstructor{},
		//eventmux:     new(event.TypeMux),
	}, nil
}

// GetAccMan retrieves the account manager.
func (n *Node) GetAccMan() *accounts.Manager {
	return n.accman
}

// GetConfig retrieves the node config.
func (n *Node) GetConfig() *NodeConfig {
	return n.config
}

// SetConfig retrieves the node config.
func (n *Node) SetConfig(cfg *NodeConfig) {
	n.config = cfg
}

// Start create a live P2P node and starts running it.
func (n *Node) Start() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Short circuit if the node's already running
	if n.client != nil {
		return ErrNodeRunning
	}
	if err := n.openDataDir(); err != nil {
		return err
	}

	// Initialize the p2p server. This creates the node key and
	// discovery databases.
	n.clientConfig = n.config.P2P
	n.clientConfig.PrivateKey = n.config.NodeKey()
	n.clientConfig.Name = n.config.NodeName()
	n.clientConfig.Logger = n.log
	if n.clientConfig.StaticNodes == nil {
		n.clientConfig.StaticNodes = n.config.StaticNodes()
	}
	if n.clientConfig.NodeDatabase == "" {
		n.clientConfig.NodeDatabase = n.config.NodeDB()
	}
	running := &p2p.Client{ClientCfg: n.clientConfig}
	log.Info("Starting peer-to-peer node", "instance", n.clientConfig.Name)

	// Otherwise copy and specialize the P2P configuration
	services := make(map[reflect.Type]FTransService)
	for _, constructor := range n.serviceFuncs {
		//Create a new context for the particular service
		ctx := &FTransServiceContext{
			Config:   n.config,
			Services: make(map[reflect.Type]FTransService),
			//EventMux: n.eventmux,
			AccountManager: n.GetAccMan(),
		}
		for kind, s := range services { // copy needed for threaded access
			ctx.Services[kind] = s
		}
		// Construct and save the service
		service, err := constructor(ctx)
		if err != nil {
			return err
		}
		kind := reflect.TypeOf(service)
		if _, exists := services[kind]; exists {
			return &DuplicateServiceError{Kind: kind}
		}
		services[kind] = service

		for _, protocol := range service.Protocols() {
			if protocol.Name == ftransfer.ProtocolName {
				if fTransAPI, ok := service.APIs()[0].Service.(*ftransfer.PrivateFTransAPI); ok {
					n.FTransAPI = fTransAPI
					n.FTransAPI.GetProtocolMan().IsClient = true
					if n.config.BasePassphrase != "" {
						if err := n.FTransAPI.GetProtocolMan().SetBaseAddress(n.config.BaseAddress); err != nil {
							log.Crit(err.Error())
						}
						n.FTransAPI.GetProtocolMan().SetBaseAddrPasswd(n.config.BasePassphrase)

						n.config.BasePassphrase = ""
					}
				}
			}
		}

	}
	// Gather the protocols and start the freshly assembled P2P server
	for _, service := range services {
		for _, p := range service.Protocols() {
			protocol := p2p.Protocol{
				Name:     p.Name,
				Version:  p.Version,
				Length:   p.Length,
				Run:      p.Run,
				NodeInfo: p.NodeInfo,
				PeerInfo: p.PeerInfo,
			}
			running.Protocols = append(running.Protocols, protocol)
		}
	}
	if err := running.Start(); err != nil {
		return convertFileLockError(err)
	}
	// Start each of the services
	started := []reflect.Type{}
	for kind, service := range services {
		// Start the next service, stopping all previous upon failure
		if err := service.StartClient(running); err != nil {
			for _, kind := range started {
				services[kind].Stop()
			}
			running.Stop()

			return err
		}
		// Mark the service started for potential cleanup
		started = append(started, kind)
	}

	// Finish initializing the startup
	n.services = services
	n.client = running
	n.stop = make(chan struct{})

	return nil
}

func (n *Node) openDataDir() error {
	if n.config.DataDir == "" {
		return nil // ephemeral
	}

	instdir := filepath.Join(n.config.DataDir, n.config.name())
	if err := os.MkdirAll(instdir, 0700); err != nil {
		return err
	}
	// Lock the instance directory to prevent concurrent use by another instance as well as
	// accidental use of the instance directory as a database.
	release, _, err := flock.New(filepath.Join(instdir, "LOCK"))
	if err != nil {
		return convertFileLockError(err)
	}
	n.instanceDirLock = release
	return nil
}

// Stop terminates a running node along with all it's services. In the node was
// not started, an error is returned.
func (n *Node) Stop() error {
	n.lock.Lock()
	defer n.lock.Unlock()

	// Short circuit if the node's not running
	if n.client == nil {
		return ErrNodeStopped
	}

	failure := &StopError{
		Services: make(map[reflect.Type]error),
	}
	for kind, service := range n.services {
		if err := service.Stop(); err != nil {
			failure.Services[kind] = err
		}
	}

	// Terminate the API, and the p2p client.
	n.client.Stop()
	n.services = nil
	n.client = nil

	// Release instance directory lock.
	if n.instanceDirLock != nil {
		if err := n.instanceDirLock.Release(); err != nil {
			n.log.Error("Can't release datadir lock", "err", err)
		}
		n.instanceDirLock = nil
	}

	// unblock n.Wait
	close(n.stop)

	if len(failure.Services) > 0 {
		return failure
	}

	return nil
}

// Register injects a new service into the node's stack. The service created by
// the passed constructor must be unique in its type with regard to sibling ones.
func (n *Node) Register(constructor FTransServiceConstructor) error {
	n.lock.Lock()
	defer n.lock.Unlock()

	if n.client != nil {
		return ErrNodeRunning
	}
	n.serviceFuncs = append(n.serviceFuncs, constructor)
	return nil
}
