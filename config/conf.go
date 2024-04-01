package config

import (
	"fmt"
	"io"

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type HealthCheck struct {
	// TODO
}

type Server struct {
	Addr string
	// net is is one of "udp", "tcp", "tcp-tls". The default value is "udp" if unset.
	Net string

	HealthCheck *HealthCheck
	// TODO: timeouts
}

type Pool struct {
	Name        string
	HealthCheck *HealthCheck
	Servers     []Server
}

type ForwardPool struct {
	Pool string
}

type Route struct {
	// either Addr or Domain must be set
	// addr can be a CIDR or a single IP
	Addr   string
	Domain string

	Forward ForwardPool
	Drop    bool
}

type Conf struct {
	LogLevel   zapcore.Level
	ListenAddr string
	// ListenNet is is one of "udp", "tcp", "tcp-tls". The default value is "udp" if unset.
	ListenNet string

	Pools  []Pool
	Routes []Route
}

func From(r io.Reader) (*Conf, error) {
	var c Conf
	if err := yaml.NewDecoder(r).Decode(&c); err != nil {
		return nil, fmt.Errorf("config: unable to load config: %w", err)
	}
	return &c, nil
}
