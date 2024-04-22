package main

import (
	"crypto/sha256"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
)

type PeerConfig struct {
	Name         string   `yaml:"name"`
	PublicKey    string   `yaml:"pub"`
	PSK          string   `yaml:"psk"`
	Source       string   `yaml:"source"`
	Destinations []string `yaml:"destination"`
}

type DarknetConfig struct {
	CIDRs []string `yaml:"clientCIDRs"`
}

type EndpointConfig struct {
	Darknet DarknetConfig `yaml:"darknet"`
}
type Config struct {
	Peers []PeerConfig `yaml:"peers"`
}

var PRIVATEKEY string
var LAST_HASH_OF_ENDPOINTYAML string

func LoadConfig() (*EndpointConfig, *Config, error) {

	// --

	prk, err := os.ReadFile("/etc/wga/endpoint/privateKey")
	if err != nil {
		return nil, nil, err
	}

	if PRIVATEKEY != "" && PRIVATEKEY != string(prk) {
		panic("private key changed, must restart wga ep")
	}
	PRIVATEKEY = string(prk)

	// --

	file, err := os.Open("/etc/wga/endpoint/endpoint.yaml")
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	hasher := sha256.New()

	var secrets = &EndpointConfig{}
	err = yaml.NewDecoder(io.TeeReader(file, hasher)).Decode(secrets)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read /etc/wga/endpoint/endpoint.yaml: %w", err)
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	if LAST_HASH_OF_ENDPOINTYAML != "" && LAST_HASH_OF_ENDPOINTYAML != hash {
		panic("hash of /etc/wga/endpoint.yaml changed, must restart wga ep")
	}
	LAST_HASH_OF_ENDPOINTYAML = hash

	// --

	var config = &Config{}
	file, err = os.Open("/etc/wga/run/run.yaml")
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	err = yaml.NewDecoder(file).Decode(config)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read /etc/wga/run/run.yaml: %w", err)
	}

	return secrets, config, nil
}
