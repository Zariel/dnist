package dnist

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"github.com/miekg/dns"
	"github.com/zariel/dnist/config"
	"go.uber.org/zap"
)

type Frontend struct {
	routes []routeMatcher
	log    *zap.Logger
}

type request struct {
	client netip.Addr
	domain string
}

type routeMatcher interface {
	Matches(r request) dns.Handler
}

type cidrMatcher struct {
	cidr netip.Prefix

	handler dns.Handler
}

func (c *cidrMatcher) Matches(req request) dns.Handler {
	if !c.cidr.Contains(req.client) {
		return nil
	}

	return c.handler
}

type domainMatcher struct {
	// regex?
	domain string

	handler dns.Handler
}

func (d *domainMatcher) Matches(req request) dns.Handler {
	// is this sufficiently correct?
	if !strings.HasSuffix(req.domain, d.domain) {
		return nil
	}
	return d.handler
}

type poolHandler struct {
	pool *config.Pool
}

type routeBuilder struct {
	conf *config.Conf
	log  *zap.Logger

	pools map[string]*downstreamPool
}

type downstreamPool struct {
	clients []*client
	log     *zap.Logger
}

func (p *downstreamPool) ServeDNS(rw dns.ResponseWriter, req *dns.Msg) {
	for _, c := range p.clients {
		resp, err := c.Send(context.TODO(), req)
		if err != nil {
			p.log.Error("unable to send request to downstream", zap.Error(err))
			continue
		}

		if err := rw.WriteMsg(resp); err != nil {
			p.log.Warn("unable to write response to client", zap.Error(err))
		}
		return
	}

	p.log.Error("unable to send request to any downstream")
	sendError(rw, req, dns.RcodeServerFailure)
}

func (b *routeBuilder) getPool(name string) (*downstreamPool, error) {
	pool, ok := b.pools[name]
	if !ok {
		return nil, fmt.Errorf("config route references unknown pool %q", name)
	}
	return pool, nil
}

func dropHandler(w dns.ResponseWriter, r *dns.Msg) {
	// TODO: what is the correct response code here?
	sendError(w, r, dns.RcodeRefused)
}

func sendError(rw dns.ResponseWriter, msg *dns.Msg, code int) {
	var answser dns.Msg
	answser.SetRcode(msg, code)
	rw.WriteMsg(&answser)
}

func (f *Frontend) ServeDNS(rw dns.ResponseWriter, msg *dns.Msg) {
	log := f.log.With(zap.Stringer("client", rw.RemoteAddr()), zap.Any("questions", msg.Question))
	log.Debug("handling request")

	if len(msg.Question) == 0 {
		log.Debug("dropping request with no questions")
		sendError(rw, msg, dns.RcodeServerFailure)
		return
	}

	addr, _, err := net.SplitHostPort(rw.RemoteAddr().String())
	if err != nil {
		log.Error("unable to parse client address", zap.Error(err))
		sendError(rw, msg, dns.RcodeServerFailure)
		return
	}

	// route based on target domain or source IP/CIDR
	client, err := netip.ParseAddr(addr)
	if err != nil {
		log.Error("unable to parse client address", zap.Error(err), zap.String("addr", addr))
		sendError(rw, msg, dns.RcodeServerFailure)
		return
	}

	req := request{
		client: netip.Addr(client),
		domain: msg.Question[0].Name,
	}

	for _, route := range f.routes {
		if handler := route.Matches(req); handler != nil {
			handler.ServeDNS(rw, msg)
			return
		}
	}

	log.Debug("no route matched")
	sendError(rw, msg, dns.RcodeServerFailure)
}
