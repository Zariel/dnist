package dnist

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/zariel/dnist/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"
)

func Run(conf *config.Conf) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	zapconf := zap.NewProductionConfig()
	zapconf.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zapconf.Level.SetLevel(conf.LogLevel)
	log, err := zapconf.Build()
	if err != nil {
		return fmt.Errorf("unable to create logger: %w", err)
	}

	routes, err := routesFromConf(ctx, conf, log)
	if err != nil {
		return fmt.Errorf("unable to build routes: %w", err)
	}

	log.Info("running with config", zap.Any("config", conf))

	var eg errgroup.Group
	eg.Go(func() error {
		if err := dns.ListenAndServe(conf.ListenAddr, conf.ListenNet, &Server{routes: routes, log: log, ctx: ctx}); err != nil {
			return fmt.Errorf("unable to start server %q: %w", conf.ListenAddr, err)
		}
		return nil
	})
	eg.Go(func() error {
		addr := conf.HTTPAddr
		if addr == "" {
			addr = "127.0.0.1:8080"
		}
		log.Info("starting http server", zap.String("addr", addr))
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			// TODO: more soffisticated health check?
			w.WriteHeader(http.StatusOK)
		})
		mux.Handle("/metrics", promhttp.Handler())
		return http.ListenAndServe(addr, mux)
	})
	return eg.Wait()
}

func routesFromConf(ctx context.Context, conf *config.Conf, log *zap.Logger) ([]routeMatcher, error) {
	pools := make(map[string]*downstreamPool, len(conf.Pools))
	for _, pool := range conf.Pools {
		s := &downstreamPool{
			clients: make([]*client, len(pool.Servers)),
		}
		// TODO: configure the pool with other options
		for i, server := range pool.Servers {
			if server.HealthCheck == nil {
				server.HealthCheck = pool.HealthCheck
			}

			client, err := newClient(ctx, pool.Name, server, log)
			if err != nil {
				// TODO: this will include dial failures which really shouldnt stop us from
				// starting the server, the pool should be marked down and we should reconnect
				return nil, fmt.Errorf("unable to create client for pool %q: %w", pool.Name, err)
			}
			s.clients[i] = client
		}
		pools[pool.Name] = s
	}

	routes := make([]routeMatcher, len(conf.Routes))
	for i, route := range conf.Routes {
		r, err := buildRoute(pools, route)
		if err != nil {
			return nil, fmt.Errorf("unable to buildconfig route %d: %w", i, err)
		}
		routes[i] = r
	}
	return routes, nil
}

func buildRoute(pools map[string]*downstreamPool, r config.Route) (routeMatcher, error) {
	var handler handler
	if r.Drop {
		handler = handlerFunc(dropHandler)
	} else {
		p, ok := pools[r.Forward.Pool]
		if !ok {
			// TODO: canonicalise the route so the config error can be more helpful
			return nil, fmt.Errorf("config route references unknown pool: %q", r.Forward.Pool)
		}
		handler = p
	}

	if r.Addr != "" {
		cidr, err := netip.ParsePrefix(r.Addr)
		if err != nil {
			return nil, fmt.Errorf("config route defines an invalid address cidr: %w", err)
		}

		return &cidrMatcher{cidr: cidr, handler: handler}, nil
	} else if r.Domain == "" {
		return nil, errors.New("config route must define either an address or a domain")
	}

	return &domainMatcher{domain: r.Domain, handler: handler}, nil
}
