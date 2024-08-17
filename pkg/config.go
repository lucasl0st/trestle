package pkg

import (
	"errors"
	"fmt"
	"github.com/go-yaml/yaml"
	"os"
)

func ParseConfig(filePath string) (*Config, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(b, &cfg)
	if err != nil {
		return nil, err
	}

	err = cfg.Validate()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

type Config struct {
	Switches []Switch `yaml:"switches"`
}

func (c Config) Validate() error {
	if len(c.Switches) == 0 {
		return errors.New("no switches defined")
	}

	for i, s := range c.Switches {
		err := s.Validate()
		if err != nil {
			return fmt.Errorf("failed to validate switch config index %d with error: %v", i, err)
		}
	}

	return nil
}

type Switch struct {
	Name       string   `yaml:"name"`
	MTU        uint16   `yaml:"mtu"`
	NetworkMTU uint16   `yaml:"network_mtu"`
	Listener   Listener `yaml:"listener"`
	Ports      []Port   `yaml:"ports"`
}

func (s Switch) Validate() error {
	if s.Name == "" {
		return errors.New("name is empty")
	}

	if s.MTU == 0 {
		return errors.New("mtu is 0")
	}

	if s.NetworkMTU == 0 {
		return errors.New("network_mtu is 0")
	}

	err := s.Listener.Validate()
	if err != nil {
		return fmt.Errorf("failed to validate listener with error: %v", err)
	}

	if len(s.Ports) == 0 {
		return errors.New("no ports defined")
	}

	for i, port := range s.Ports {
		err = port.Validate()
		if err != nil {
			return fmt.Errorf("failed to validate port at index %d with error: %v", i, err)
		}
	}

	return nil
}

type Listener struct {
	Hostname string `yaml:"hostname"`
	Port     uint16 `yaml:"port"`
}

func (l Listener) Validate() error {
	if l.Hostname == "" {
		return errors.New("hostname is empty")
	}

	return nil
}

type Port struct {
	TAPNIC TAPNIC `yaml:"tapnic"`
	Peer   Peer   `yaml:"peer"`
}

func (p Port) Validate() error {
	if p.TAPNIC.Name != "" && p.Peer.Name != "" {
		return errors.New("both tapnic and peer defined, choose one")
	}

	if p.TAPNIC.Name != "" {
		err := p.TAPNIC.Validate()
		if err != nil {
			return fmt.Errorf("failed to validate tapnic with error: %v", err)
		}

		return nil
	}

	if p.Peer.Name != "" {
		err := p.Peer.Validate()
		if err != nil {
			return fmt.Errorf("failed to validate peer with error: %v", err)
		}

		return nil
	}

	return errors.New("neither tapnic or peer name defined")
}

type TAPNIC struct {
	Name string `yaml:"name"`
}

func (t TAPNIC) Validate() error {
	if t.Name == "" {
		return errors.New("name is empty")
	}

	return nil
}

type Peer struct {
	Name     string `yaml:"name"`
	Hostname string `yaml:"hostname"`
	Port     uint16 `yaml:"port"`
}

func (p Peer) Validate() error {
	if p.Name == "" {
		return errors.New("name is empty")
	}

	if p.Hostname == "" {
		return errors.New("hostname is empty")
	}

	if p.Port == 0 {
		return errors.New("port is 0")
	}

	return nil
}
