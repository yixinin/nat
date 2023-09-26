package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Backend  *BackendConfig  `yaml:"backend"`
	Frontend *FrontendConfig `yaml:"frontend"`
	Server   *ServerConfig   `yaml:"server"`
	Quic     *QuicConfig     `yaml:"quic"`
}

type BackendConfig struct {
	Addr     string `yaml:"addr"`
	FQDN     string `yaml:"fqdn"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
	StunAddr string `yaml:"stun_addr"`
}
type FrontendConfig struct {
	Addr     string   `yaml:"addr"`
	FQDN     []string `yaml:"fqdn"`
	StunAddr string   `yaml:"stun_addr"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
	DDNS bool   `yaml:"ddns"`
}

type QuicConfig struct {
	Port     uint16 `yaml:"port"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var config = new(Config)
	return config, yaml.Unmarshal(data, config)
}
