package main

import (
	"github.com/lucasl0st/trestle/internal"
	"github.com/lucasl0st/trestle/pkg"
	"os"
)

const configEnv = "CONFIG"
const defaultConfigPath = "config.yaml"

func main() {
	configPath := os.Getenv(configEnv)
	if configPath == "" {
		configPath = defaultConfigPath
	}

	cfg, err := pkg.ParseConfig(configPath)
	if err != nil {
		panic(err)
	}

	var listeners []internal.Listener

	for _, s := range cfg.Switches {
		sw := internal.NewSwitch(s.Name)
		l, err := internal.NewListener(s.Listener.Hostname, s.Listener.Port, s.MTU, s.NetworkMTU, sw)
		if err != nil {
			panic(err)
		}

		for _, p := range s.Ports {
			if p.TAPNIC.Name != "" {
				i, err := internal.NewTAPNIC(p.TAPNIC.Name, s.MTU)
				if err != nil {
					panic(err)
				}

				sw.AddPort(i)
				continue
			}

			err = l.Connect(p.Peer.Hostname, p.Peer.Port)
			if err != nil {
				panic(err)
			}
		}

		listeners = append(listeners, l)
	}

	for _, listener := range listeners {
		go func() {
			err = listener.Listen()
			if err != nil {
				panic(err)
			}
		}()
	}

	select {}
}
