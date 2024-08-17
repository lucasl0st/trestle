package internal

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/lucasl0st/trestle/internal/util"
	"github.com/lucasl0st/trestle/pkg/packet"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"net"
)

type Listener interface {
	Listen() error
	Connect(hostname string, port uint16) error
	Read(peerId string) (*packet.Packet, error)
	Write(peerId string, packet *packet.Packet) error
	Close() error
}

type PeerReceiver interface {
	AddPort(port Port) uint
}

type listener struct {
	mtu        uint16
	networkMTU uint16
	alive      bool
	conn       *net.UDPConn

	// udp address -> peerId
	addressToPeerId *util.SafeMap[string, string]
	// peerId -> udp address
	peerIdToAddress *util.SafeMap[string, string]

	// peerId -> package queue
	incomingPackages *util.SafeMap[string, *util.Queue[*packet.Packet]]

	receiver PeerReceiver
}

func NewListener(hostname string, port uint16, mtu uint16, networkMTU uint16, receiver PeerReceiver) (Listener, error) {
	listenAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", hostname, port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return nil, err
	}

	return &listener{
		mtu:              mtu,
		networkMTU:       networkMTU,
		alive:            true,
		conn:             conn,
		addressToPeerId:  util.NewSafeMap[string, string](),
		peerIdToAddress:  util.NewSafeMap[string, string](),
		incomingPackages: util.NewSafeMap[string, *util.Queue[*packet.Packet]](),
		receiver:         receiver,
	}, nil
}

func (l *listener) Listen() error {
	for l.alive {
		buf := make([]byte, l.networkMTU)
		n, addr, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}

		var p packet.Packet
		err = proto.Unmarshal(buf[:n], &p)
		if err != nil {
			slog.Error("could not unmarshal packet", "addr", addr.String(), "error", err)
			continue
		}

		if p.Type == packet.PacketType_ACK_SESSION {
			err = l.ackSession(&p, addr)
			if err != nil {
				slog.Error("failed to ack session", "addr", addr.String(), err)
			}

			continue
		}

		if p.Type == packet.PacketType_INITIATE_SESSION {
			err = l.initiateSession(&p, addr)
			if err != nil {
				slog.Error("failed to initiate session", "addr", addr.String(), err)
			}

			continue
		}

		peerId, ok := l.addressToPeerId.Get(addr.String())
		if !ok || p.Type == packet.PacketType_HELO {
			err = l.establishSession(addr)
			if err != nil {
				slog.Error("could not establish session", "addr", addr.String(), "error", err)
			}

			continue
		}

		queue, ok := l.incomingPackages.Get(peerId)
		if !ok {
			continue
		}

		queue.Add(&p)
	}

	return nil
}

func (l *listener) Connect(hostname string, port uint16) error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", hostname, port))
	if err != nil {
		return err
	}

	p := packet.Packet{
		Type:    packet.PacketType_HELO,
		Payload: &packet.Packet_Helo{},
	}

	b, err := proto.Marshal(&p)
	if err != nil {
		return err
	}

	_, err = l.conn.WriteToUDP(b, addr)
	return err
}

func (l *listener) establishSession(addr *net.UDPAddr) error {
	p := packet.Packet{
		Type: packet.PacketType_INITIATE_SESSION,
		Payload: &packet.Packet_InitiateSession{
			InitiateSession: &packet.InitiateSession{
				Mtu:        uint32(l.mtu),
				NetworkMtu: uint32(l.networkMTU),
			},
		},
	}

	b, err := proto.Marshal(&p)
	if err != nil {
		return err
	}

	_, err = l.conn.WriteToUDP(b, addr)
	return err
}

func (l *listener) initiateSession(p *packet.Packet, addr *net.UDPAddr) error {
	payload := p.Payload.(*packet.Packet_InitiateSession)
	if payload.InitiateSession.Mtu != uint32(l.mtu) {
		return fmt.Errorf("session mtu %d must be the same as configured mtu %d", payload.InitiateSession.Mtu, l.mtu)
	}

	if payload.InitiateSession.NetworkMtu != uint32(l.networkMTU) {
		return fmt.Errorf("session network mtu %d must be the same as configured network mtu %d", payload.InitiateSession.NetworkMtu, l.networkMTU)
	}

	p = &packet.Packet{
		Type: packet.PacketType_ACK_SESSION,
		Payload: &packet.Packet_AckSession{
			AckSession: &packet.AckSession{
				Id: uuid.New().String(),
			},
		},
	}

	b, err := proto.Marshal(p)
	if err != nil {
		return err
	}

	_, err = l.conn.WriteToUDP(b, addr)
	if err != nil {
		return err
	}

	_, ok := l.addressToPeerId.Get(addr.String())
	if !ok {
		return l.establishSession(addr)
	}

	return nil
}

func (l *listener) ackSession(p *packet.Packet, addr *net.UDPAddr) error {
	payload, ok := p.Payload.(*packet.Packet_AckSession)
	if !ok {
		return errors.New("message was ACK_SESSION but payload type is invalid")
	}

	peerId := payload.AckSession.Id

	l.addressToPeerId.Set(addr.String(), peerId)
	l.peerIdToAddress.Set(peerId, addr.String())
	l.incomingPackages.Set(peerId, util.NewQueue[*packet.Packet](512))

	peer := NewPeer(l, peerId, uint32(l.networkMTU))
	l.receiver.AddPort(peer)
	return nil
}

func (l *listener) Read(peerId string) (*packet.Packet, error) {
	queue, ok := l.incomingPackages.Get(peerId)
	if !ok {
		return nil, errors.New("session not established")
	}

	return queue.Grab(), nil
}

func (l *listener) Write(peerId string, packet *packet.Packet) error {
	udpAddr, ok := l.peerIdToAddress.Get(peerId)
	if !ok {
		return errors.New("peer not found")
	}

	peerAddr, err := net.ResolveUDPAddr("udp", udpAddr)
	if err != nil {
		return err
	}

	b, err := proto.Marshal(packet)
	if err != nil {
		return err
	}

	_, err = l.conn.WriteToUDP(b, peerAddr)
	return err
}

func (l *listener) Close() error {
	l.alive = false
	return nil
}
