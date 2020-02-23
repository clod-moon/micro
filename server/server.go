package server

import (
	"fmt"
	"os"
	"time"

	"github.com/micro/cli/v2"
	"github.com/micro/go-micro/v2"
	log "github.com/micro/go-micro/v2/logger"
	"github.com/micro/go-micro/v2/router"
	"github.com/micro/go-micro/v2/server"
	"github.com/micro/go-micro/v2/transport"
	"github.com/micro/go-micro/v2/transport/grpc"
)

var (
	// Name of the server microservice
	Name = "go.micro.server"
	// Address is the router microservice bind address
	Address = ":8087"
	// Router is the router address a.k.a. gossip address
	Router = ":9093"
	// Network is the router network id
	Network = "local"
)

// srv is micro server
type srv struct {
	// router is micro router
	router router.Router
	// network is micro network server
	network server.Server
}

// newServer creates new micro server and returns it
func newServer(s micro.Service, r router.Router) *srv {
	// NOTE: this will end up being QUIC transport once it gets stable
	t := grpc.NewTransport(transport.Addrs(Network))
	n := server.NewServer(server.Transport(t))

	return &srv{
		router:  r,
		network: n,
	}
}

// start starts the micro server.
func (s *srv) start() error {
	log.Info("starting micro server")

	// start the router
	if err := s.router.Start(); err != nil {
		return err
	}

	return nil
}

// stop stops the micro server.
func (s *srv) stop() error {
	log.Info("stopping server")

	// stop the router
	if err := s.router.Stop(); err != nil {
		return fmt.Errorf("failed to stop router: %v", err)
	}

	return nil
}

// run runs the micro server
func run(ctx *cli.Context, srvOpts ...micro.Option) {
	log.Init(log.WithFields(map[string]interface{}{"service": "server"}))

	// Init plugins
	for _, p := range Plugins() {
		p.Init(ctx)
	}

	if len(ctx.String("server_name")) > 0 {
		Name = ctx.String("server_name")
	}
	if len(ctx.String("address")) > 0 {
		Address = ctx.String("address")
	}
	if len(ctx.String("router_address")) > 0 {
		Router = ctx.String("router")
	}
	if len(ctx.String("network_address")) > 0 {
		Network = ctx.String("network")
	}

	// Initialise service
	service := micro.NewService(
		micro.Name(Name),
		micro.Address(Address),
		micro.RegisterTTL(time.Duration(ctx.Int("register_ttl"))*time.Second),
		micro.RegisterInterval(time.Duration(ctx.Int("register_interval"))*time.Second),
	)

	// create new router
	r := router.NewRouter(
		router.Id(service.Server().Options().Id),
		router.Address(Router),
		router.Network(Network),
		router.Registry(service.Client().Options().Registry),
	)

	// create new server and start it
	s := newServer(service, r)

	if err := s.start(); err != nil {
		log.Errorf("failed to start: %s", err)
		os.Exit(1)
	}

	log.Info("successfully started")

	if err := service.Run(); err != nil {
		log.Errorf("failed with error %s", err)
		// TODO: we should probably stop the router here before bailing
		os.Exit(1)
	}

	// stop the server
	if err := s.stop(); err != nil {
		log.Errorf("failed to stop: %v", err)
		os.Exit(1)
	}

	log.Info("successfully stopped")
}

func Commands(options ...micro.Option) []*cli.Command {
	command := &cli.Command{
		Name:  "server",
		Usage: "Run the micro network server",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "address",
				Usage:   "Set the micro server address :8083",
				EnvVars: []string{"MICRO_SERVER_ADDRESS"},
			},
			&cli.StringFlag{
				Name:    "router_address",
				Usage:   "Set the micro router address :9093",
				EnvVars: []string{"MICRO_ROUTER_ADDRESS"},
			},
			&cli.StringFlag{
				Name:    "network_address",
				Usage:   "Set the micro network id :local",
				EnvVars: []string{"MICRO_NETWORK_ADDRESS"},
			},
		},
		Action: func(ctx *cli.Context) error {
			run(ctx, options...)
			return nil
		},
	}

	for _, p := range Plugins() {
		if cmds := p.Commands(); len(cmds) > 0 {
			command.Subcommands = append(command.Subcommands, cmds...)
		}

		if flags := p.Flags(); len(flags) > 0 {
			command.Flags = append(command.Flags, flags...)
		}
	}

	return []*cli.Command{command}
}
