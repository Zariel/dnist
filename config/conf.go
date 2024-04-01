package config

import (
	"fmt"
	"io"
	"time"

	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v3"
)

type HealthCheck struct {
	CheckName        string `yaml:"checkName"`
	Timeout          time.Duration
	Interval         time.Duration
	SuccessThreshold int `yaml:"successThreshold"`
	FailureThreshold int `yaml:"failureThreshold"`
}

type Server struct {
	Addr string
	// net is is one of "udp", "tcp", "tcp-tls". The default value is "udp" if unset.
	Net string

	// optional timeout
	Timeout time.Duration

	HealthCheck *HealthCheck
}

type Pool struct {
	Name    string
	Servers []Server
	// if the server does not specify a health check, this one will be used
	HealthCheck *HealthCheck
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

	HTTPAddr string `yaml:"httpAddr"`

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
