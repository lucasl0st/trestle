package internal

import "github.com/songgao/packets/ethernet"

type Port interface {
	Write(frame ethernet.Frame) error
	Read() (ethernet.Frame, error)
	Close() error
}
