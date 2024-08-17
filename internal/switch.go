package internal

import (
	"bytes"
	"github.com/lucasl0st/trestle/internal/util"
	"github.com/songgao/packets/ethernet"
	"log/slog"
)

type Switch interface {
	AddPort(port Port) uint
	RemovePort(uint)
	Close() error
}

type ethernetSwitch struct {
	name string

	ports      *util.SafeMap[uint, Port]
	portActive *util.SafeMap[uint, bool]

	hardwareAddr   *util.SafeMap[string, uint]
	outgoingFrames *util.SafeMap[uint, *util.Queue[ethernet.Frame]]

	portId uint
}

var broadcastMac = []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

func NewSwitch(name string) Switch {
	return &ethernetSwitch{
		name:           name,
		ports:          util.NewSafeMap[uint, Port](),
		portActive:     util.NewSafeMap[uint, bool](),
		hardwareAddr:   util.NewSafeMap[string, uint](),
		outgoingFrames: util.NewSafeMap[uint, *util.Queue[ethernet.Frame]](),
	}
}

func (e *ethernetSwitch) AddPort(port Port) uint {
	portId := e.portId
	e.portId++

	e.portActive.Set(portId, true)
	e.outgoingFrames.Set(portId, util.NewQueue[ethernet.Frame](500))

	go e.read(port, portId)
	go e.write(port, portId)

	slog.Info("added port", "switch", e.name, "portId", portId)
	return portId
}

func (e *ethernetSwitch) RemovePort(portId uint) {
	e.portActive.Set(portId, false)
	e.outgoingFrames.Delete(portId)

	port, ok := e.ports.Get(portId)
	if !ok {
		return
	}

	err := port.Close()
	if err != nil {
		slog.Error("failed to close port", "switch", e.name, "portId", portId, "error", err)
	}

	slog.Info("removed port", "switch", e.name, "portId", portId)
}

func (e *ethernetSwitch) read(port Port, portId uint) {
	for {
		active, ok := e.portActive.Get(portId)
		if !ok || !active {
			return
		}

		frame, err := port.Read()
		if err != nil {
			slog.Error("failed to read frame of port", "switch", e.name, "portId", portId, "error", err)
			e.RemovePort(portId)
			return
		}

		e.transportFrame(frame, portId)
	}
}

func (e *ethernetSwitch) write(port Port, portId uint) {
	queue, ok := e.outgoingFrames.Get(portId)
	if !ok {
		panic("queue not found")
	}

	for {
		active, ok := e.portActive.Get(portId)
		if !ok || !active {
			return
		}

		frame := queue.Grab()
		err := port.Write(frame)
		if err != nil {
			slog.Error("failed to write frame to port", "switch", e.name, "portId", portId, "error", err)
			e.RemovePort(portId)
			return
		}
	}
}

func (e *ethernetSwitch) transportFrame(frame ethernet.Frame, sourcePortId uint) {
	e.hardwareAddr.Set(frame.Source().String(), sourcePortId)

	if bytes.Equal(frame.Destination(), broadcastMac) {
		e.broadcastFrame(frame, sourcePortId)
		return
	}

	e.singlecastFrame(frame, sourcePortId)
}

func (e *ethernetSwitch) broadcastFrame(frame ethernet.Frame, sourcePortId uint) {
	e.outgoingFrames.Range(func(portId uint, queue *util.Queue[ethernet.Frame]) bool {
		if portId == sourcePortId {
			return true
		}

		queue.Add(frame)
		return true
	})
}

func (e *ethernetSwitch) singlecastFrame(frame ethernet.Frame, sourcePortId uint) {
	targetPortId, ok := e.hardwareAddr.Get(frame.Destination().String())
	if !ok {
		return
	}

	if targetPortId == sourcePortId {
		return
	}

	queue, ok := e.outgoingFrames.Get(targetPortId)
	if !ok {
		return
	}

	queue.Add(frame)
}

func (e *ethernetSwitch) Close() error {
	e.ports.Range(func(portId uint, port Port) bool {
		e.RemovePort(portId)
		return true
	})

	return nil
}
