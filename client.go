package dnist

import (
	"context"
	"fmt"
	"net"

	"github.com/miekg/dns"
	"github.com/zariel/dnist/config"
	"go.uber.org/zap"
)

type client struct {
	c    *dns.Client
	addr string
	conn *dns.Conn
	log  *zap.Logger
}

func newClient(conf config.Server, log *zap.Logger) (*client, error) {
	var c dns.Client
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

	conn, err := c.DialContext(context.TODO(), addr)
	if err != nil {
		return nil, fmt.Errorf("unable to dial server %q: %w", addr, err)
	}
	return &client{
		c:    &c,
		conn: conn,
		addr: addr,
		log:  log.With(zap.String("upstream", addr)),
	}, nil
}

func (c *client) Close() error {
	return c.conn.Close()
}

func (c *client) Send(ctx context.Context, m *dns.Msg) (*dns.Msg, error) {
	c.log.Debug("sending query", zap.Any("msg", m))
	resp, _, err := c.c.ExchangeWithConnContext(ctx, m, c.conn)
	return resp, err
}
