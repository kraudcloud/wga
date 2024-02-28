package main

import (
	"encoding/json"
	"os"
)

type PrintConfig struct {
	PublicEndpoint string `json:"publicEndpoint"`
}

type ServerConfig struct {
	PrivateKey string   `json:"privateKey"` //TODO k8s secret
	IPs        []string `json:"ips"`
}

type PeerConfig struct {
	Name       string   `json:"name"`
	PrivateKey string   `json:"privateKey"`
	IPs        []string `json:"ips"`
	Routes     []string `json:"routes"`
}

type Config struct {
	AllPeers PeerConfig   `json:"allPeers"`
	Print    PrintConfig  `json:"print"`
	Server   ServerConfig `json:"server"`
	Peers    []PeerConfig `json:"peers"`
}

func LoadConfig() (*Config, error) {
	file, err := os.Open("/etc/wga/config.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	config := Config{}
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	for i, _ := range config.Peers {
		config.Peers[i].Routes = append(config.AllPeers.Routes, config.Peers[i].Routes...)
	}

	return &config, nil
}
