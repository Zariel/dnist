package dnist

import (
	"context"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/zariel/dnist/config"
)

type responseWriter struct {
	Resp dns.Msg
}

// Close implements dns.ResponseWriter.
func (r *responseWriter) Close() error {
	return nil
}

// Hijack implements dns.ResponseWriter.
func (r *responseWriter) Hijack() {
	panic("unimplemented")
}

// LocalAddr implements dns.ResponseWriter.
func (r *responseWriter) LocalAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 1234,
	}
}

// RemoteAddr implements dns.ResponseWriter.
func (r *responseWriter) RemoteAddr() net.Addr {
	return &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 55555,
	}
}

// TsigStatus implements dns.ResponseWriter.
func (r *responseWriter) TsigStatus() error {
	panic("unimplemented")
}

// TsigTimersOnly implements dns.ResponseWriter.
func (r *responseWriter) TsigTimersOnly(bool) {
	panic("unimplemented")
}

// Write implements dns.ResponseWriter.
func (r *responseWriter) Write(p []byte) (int, error) {
	if err := r.Resp.Unpack(p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteMsg implements dns.ResponseWriter.
func (r *responseWriter) WriteMsg(m *dns.Msg) error {
	m.CopyTo(&r.Resp)
	return nil
}

func TestBuildRouteDrop(t *testing.T) {
	r, err := buildRoute(nil, config.Route{Domain: "example.com", Drop: true})
	if err != nil {
		t.Fatal(err)
	}

	handler := r.Matches(request{domain: "example.com"})
	if handler == nil {
		t.Fatal("expected handler")
	}

	var (
		msg dns.Msg
		rw  responseWriter
	)
	handler.ServeDNS(context.Background(), &rw, msg.SetQuestion("example.com.", dns.TypeA))

	if rw.Resp.Rcode != dns.RcodeRefused {
		t.Fatalf("unexpected response code: %v", rw.Resp.Rcode)
	}
}
