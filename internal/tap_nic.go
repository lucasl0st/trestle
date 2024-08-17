package internal

import (
	"github.com/milosgajdos/tenus"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
)

type tapNic struct {
	mtu uint16

	nic *water.Interface
}

func NewTAPNIC(name string, mtu uint16) (Port, error) {
	cfg := water.Config{
		PlatformSpecificParams: water.PlatformSpecificParams{
			Name: name,
		},
		DeviceType: water.TAP,
	}

	i, err := water.New(cfg)
	if err != nil {
		return nil, err
	}

	link, err := tenus.NewLinkFrom(i.Name())
	if err != nil {
		return nil, err
	}

	err = link.SetLinkMTU(int(mtu))
	if err != nil {
		return nil, err
	}

	err = link.SetLinkUp()
	if err != nil {
		return nil, err
	}

	return &tapNic{
		mtu: mtu,
		nic: i,
	}, nil
}

func (n *tapNic) Write(frame ethernet.Frame) error {
	_, err := n.nic.Write(frame)
	return err
}

func (n *tapNic) Read() (ethernet.Frame, error) {
	frame := ethernet.Frame{}
	frame.Resize(int(n.mtu))

	c, err := n.nic.Read(frame)
	if err != nil {
		return nil, err
	}

	return frame[:c], nil
}

func (n *tapNic) Close() error {
	return n.nic.Close()
}
