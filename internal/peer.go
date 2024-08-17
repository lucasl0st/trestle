package internal

import (
	"github.com/lucasl0st/trestle/internal/util"
	"github.com/lucasl0st/trestle/pkg/packet"
	"github.com/songgao/packets/ethernet"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"math"
	"sort"
)

type peer struct {
	listener Listener
	id       string

	maxPayloadSize    int
	fragmentedPackets *util.SafeMap[uint32, []*packet.Packet]

	packedId uint32
}

func maxPayloadSize(networkMTU uint32) int {
	p := &packet.Packet{
		Type: packet.PacketType_FRAGMENTED_DATA,
		Payload: &packet.Packet_FragmentedData{
			FragmentedData: &packet.FragmentedData{
				Id:          math.MaxInt32,
				Fragment:    math.MaxInt32,
				FragmentMax: math.MaxInt32,
			},
		},
	}

	b, err := proto.Marshal(p)
	if err != nil {
		panic(err)
	}

	return int(networkMTU - uint32(len(b)))
}

func NewPeer(listener Listener, id string, networkMTU uint32) Port {
	maxPayloadSize := maxPayloadSize(networkMTU)
	slog.Info("calculated max payload size", "size", maxPayloadSize)

	return &peer{
		listener:          listener,
		id:                id,
		maxPayloadSize:    maxPayloadSize,
		fragmentedPackets: util.NewSafeMap[uint32, []*packet.Packet](),
	}
}

func (p *peer) Write(frame ethernet.Frame) error {
	fragments := p.fragment(frame)

	for _, fragment := range fragments {
		err := p.listener.Write(p.id, fragment)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *peer) Read() (ethernet.Frame, error) {
	pack, err := p.listener.Read(p.id)
	if err != nil {
		return nil, err
	}

	payload := pack.Payload.(*packet.Packet_FragmentedData)
	fragments, _ := p.fragmentedPackets.Get(payload.FragmentedData.Id)
	fragments = append(fragments, pack)
	p.fragmentedPackets.Set(payload.FragmentedData.Id, fragments)

	frame := p.findCompleteFrame()
	if frame == nil {
		return p.Read()
	}

	return frame, nil
}

func (p *peer) fragment(frame ethernet.Frame) []*packet.Packet {
	packetId := p.packedId
	p.packedId++

	var fragments []*packet.Packet
	var k uint32 = 0

	for i := 0; i < len(frame); i += p.maxPayloadSize {
		end := i + p.maxPayloadSize
		if end > len(frame) {
			end = len(frame)
		}

		fragment := &packet.Packet{
			Type: packet.PacketType_FRAGMENTED_DATA,
			Payload: &packet.Packet_FragmentedData{
				FragmentedData: &packet.FragmentedData{
					Id:       packetId,
					Fragment: k,
					Payload:  frame[i:end],
				},
			},
		}

		fragments = append(fragments, fragment)
		k++
	}

	for _, fragment := range fragments {
		fragment.Payload.(*packet.Packet_FragmentedData).FragmentedData.FragmentMax = k
	}

	return fragments
}

func (p *peer) findCompleteFrame() ethernet.Frame {
	var toDeFragment []*packet.Packet
	toDeFragmentPacketid := uint32(0)

	p.fragmentedPackets.Range(func(packetId uint32, fragments []*packet.Packet) bool {
		if len(fragments) == 0 {
			return true
		}

		if uint32(len(fragments)) >= fragments[0].Payload.(*packet.Packet_FragmentedData).FragmentedData.FragmentMax {
			toDeFragment = fragments
			toDeFragmentPacketid = packetId
			return false
		}

		return true
	})

	if toDeFragment == nil {
		return nil
	}

	return p.deFragmentFrame(toDeFragmentPacketid, toDeFragment)
}

func (p *peer) deFragmentFrame(packetId uint32, fragments []*packet.Packet) ethernet.Frame {
	sort.Slice(fragments, func(i, j int) bool {
		a := fragments[i].Payload.(*packet.Packet_FragmentedData)
		b := fragments[j].Payload.(*packet.Packet_FragmentedData)

		return a.FragmentedData.Fragment < b.FragmentedData.Fragment
	})

	var frame ethernet.Frame

	for _, fragment := range fragments {
		frame = append(frame, fragment.Payload.(*packet.Packet_FragmentedData).FragmentedData.Payload...)
	}

	p.fragmentedPackets.Delete(packetId)
	return frame
}

func (p *peer) Close() error {
	return nil
}
