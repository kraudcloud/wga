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
	PublicKey    string   `yaml:"publicKey"`
	PSK          string   `yaml:"presharedKey"`
	Source       string   `yaml:"source"`
	Destinations []string `yaml:"destination"`
}

type DarknetConfig struct {
	CIDRs []string `yaml:"cidrs"`
}

type Secrets struct {
	PrivateKey string        `yaml:"privateKey"`
	Darknet    DarknetConfig `yaml:"darknet"`
}
type Config struct {
	Peers []PeerConfig `yaml:"peers"`
}

var LAST_HASH_OF_SECRETS string

func LoadConfig() (*Secrets, *Config, error) {

	file, err := os.Open("/etc/wga/secrets.yaml")
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	hasher := sha256.New()

	var secrets = &Secrets{}
	err = yaml.NewDecoder(io.TeeReader(file, hasher)).Decode(secrets)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read /etc/wga/secrets.yaml: %w", err)
	}

	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	if LAST_HASH_OF_SECRETS != "" && LAST_HASH_OF_SECRETS != hash {
		panic("hash of /etc/wga/secrets.yaml changed, must restart wga ep")
	}
	LAST_HASH_OF_SECRETS = hash

	var config = &Config{}
	file, err = os.Open("/etc/wga/config.yaml")
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	err = yaml.NewDecoder(file).Decode(config)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot read /etc/wga/config.yaml: %w", err)
	}

	return secrets, config, nil
}
