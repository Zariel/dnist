package dnist

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zariel/dnist/config"
	"go.uber.org/zap"
)

func registerMetric[T prometheus.Collector](m T) T {
	prometheus.MustRegister(m)
	return m
}

var (
	poolLatency = registerMetric(prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "dnist",
		Subsystem: "client",
		Name:      "pool_latency_seconds",
		Help:      "Latency of client pool requests",
	}, []string{"pool"}))

	clientResponses = registerMetric(prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "dnist",
		Subsystem: "client",
		Name:      "responses_total",
		Help:      "Total number of responses by code",
	}, []string{"pool", "rcode"}))
)

type checkClient interface {
	Send(context.Context, *dns.Msg) (*dns.Msg, error)
}

type healthCheck struct {
	conf config.HealthCheck

	client checkClient

	mu       sync.Mutex
	failures int
	success  int
	up       bool
}

func (h *healthCheck) check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, h.conf.Timeout)
	defer cancel()

	var msg dns.Msg
	_, err := h.client.Send(ctx, msg.SetQuestion(h.conf.CheckName, dns.TypeA))
	return err
}

func (h *healthCheck) run(ctx context.Context, log *zap.Logger) {
	timer := time.NewTimer(h.conf.Interval)
	defer timer.Stop()

	log = log.With(zap.String("checkname", h.conf.CheckName))

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		err := h.check(ctx)
		if err != nil {
			log.Info("error response to health check", zap.Error(err))
		}

		var success, failures int
		h.mu.Lock()
		if err != nil {
			h.failures++
			h.success = 0
			failures = h.failures
		} else {
			h.success++
			h.failures = 0
			success = h.success
		}

		if h.up && failures >= h.conf.FailureThreshold {
			h.up = false
			log.Warn("marking client DOWN due to health check failures", zap.Int("failures", failures))
		} else if !h.up && success >= h.conf.SuccessThreshold {
			h.up = true
			log.Info("marking client UP", zap.Int("success", success))
		}
		h.mu.Unlock()

		timer.Reset(h.conf.Interval)
	}
}

func (h *healthCheck) isUp() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.up
}

type client struct {
	c    *dns.Client
	addr string
	conn *dns.Conn
	log  *zap.Logger

	poolName string

	health *healthCheck
}

func newClient(ctx context.Context, poolName string, conf config.Server, log *zap.Logger) (*client, error) {
	c := &dns.Client{
		Timeout: conf.Timeout,
	}

	switch conf.Net {
	case "udp", "":
		c.Net = "udp"
	case "tcp":
		c.Net = "tcp"
	case "tcp-tls":
		c.Net = "tcp-tls"
	default:
		return nil, fmt.Errorf("server %s: invalid net %q", conf.Addr, conf.Net)
	}

	addr := conf.Addr
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "53")
	}

	log.With(zap.String("addr", addr))

	conn, err := c.DialContext(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("unable to dial server %q: %w", addr, err)
	}

	client := &client{
		c:        c,
		conn:     conn,
		addr:     addr,
		log:      log,
		poolName: poolName,
	}

	if conf.HealthCheck != nil {
		client.health = &healthCheck{
			conf:   *conf.HealthCheck,
			client: client,
			up:     true,
		}
		go client.health.run(ctx, log)
	}

	return client, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) Send(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
	timer := prometheus.NewTimer(poolLatency.WithLabelValues(c.poolName))
	defer timer.ObserveDuration()

	q := m.Question[0]
	c.log.Debug("sending query", zap.String("query", q.Name), zap.Stringer("type", dns.Type(q.Qtype)), zap.Stringer("class", dns.Class(q.Qclass)))
	resp, _, err := c.c.ExchangeWithConnContext(ctx, m, c.conn)
	if err != nil {
		clientResponses.WithLabelValues(c.poolName, "error").Inc()
	} else {
		clientResponses.WithLabelValues(c.poolName, dns.RcodeToString[resp.Rcode]).Inc()
	}
	return resp, err
}
