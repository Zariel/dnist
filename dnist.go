package dnist

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/miekg/dns"
	"github.com/zariel/dnist/config"
	"go.uber.org/zap"
)

func Run(conf *config.Conf) error {
	zapconf := zap.NewProductionConfig()
	zapconf.Level.SetLevel(conf.LogLevel)
	log, err := zapconf.Build()
	if err != nil {
		return fmt.Errorf("unable to create logger: %w", err)
	}

	routes, err := routesFromConf(conf, log)
	if err != nil {
		return fmt.Errorf("unable to build routes: %w", err)
	}

	log.Info("running with config", zap.Any("config", conf))
	if err := dns.ListenAndServe(conf.ListenAddr, conf.ListenNet, &Frontend{routes: routes, log: log}); err != nil {
		return fmt.Errorf("unable to start server %q: %w", conf.ListenAddr, err)
	}
	return nil
}

func routesFromConf(conf *config.Conf, log *zap.Logger) ([]routeMatcher, error) {
	pools := make(map[string]*downstreamPool, len(conf.Pools))
	for _, pool := range conf.Pools {
		s := &downstreamPool{
			clients: make([]*client, len(pool.Servers)),
		}
		// TODO: configure the pool with other options
		for i, server := range pool.Servers {
			client, err := newClient(server, log)
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
	p, ok := pools[r.Forward.Pool]
	if !ok {
		// TODO: canonicalise the route so the config error can be more helpful
		return nil, fmt.Errorf("config route references unknown pool: %q", r.Forward.Pool)
	}

	if r.Addr != "" {
		cidr, err := netip.ParsePrefix(r.Addr)
		if err != nil {
			return nil, fmt.Errorf("config route defines an invalid address cidr: %w", err)
		}
		if r.Drop {
			return &cidrMatcher{cidr, dns.HandlerFunc(dropHandler)}, nil
		}

		return &cidrMatcher{cidr: cidr, handler: p}, nil
	} else if r.Domain == "" {
		return nil, errors.New("config route must define either an address or a domain")
	}

	return &domainMatcher{domain: r.Domain, handler: p}, nil
}
