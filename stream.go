package main

import (
	"fmt"
	"log"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type packetInfo struct {
	p   gopacket.Packet
	ip  *layers.IPv4
	tcp *layers.TCP
}

type stream struct {
	b              []byte
	ch             chan packetInfo
	info           packetInfo
	packets        int // read
	packetsWritten int
	seq            []uint32
}

func NewStream() *stream {
	s := &stream{
		ch:  make(chan packetInfo, 10),
		seq: make([]uint32, 0, 1024),
	}
	return s
}

func (s *stream) Write(packet gopacket.Packet) {
	if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
		log.Println("Unusable packet")
		return
	}

	// t := packet.Metadata().Timestamp
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return
	}
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return
	}

	ip, _ := ipLayer.(*layers.IPv4)
	tcp, _ := tcpLayer.(*layers.TCP)

	if len(tcp.LayerPayload()) > 0 {
		// fmt.Printf("%s %s:%d -> %s:%d seq %5d\n", t.Format("15:04:05.000"), ip.SrcIP, tcp.SrcPort, ip.DstIP, tcp.DstPort, tcp.Seq)
		for _, seq := range s.seq {
			if seq == tcp.Seq {
				fmt.Fprintf(os.Stderr, "ignoring duplicate seq %d\n", tcp.Seq)
				return
			}
		}
		s.seq = append(s.seq, tcp.Seq)
		if len(s.seq) > 1000 {
			s.seq = s.seq[len(s.seq)-1000:]
		}

		s.ch <- packetInfo{
			p:   packet,
			ip:  ip,
			tcp: tcp,
		}
	}
}

func (s *stream) Read(p []byte) (n int, err error) {
	if len(s.b) == 0 {
		s.info = <-s.ch
		s.b = s.info.tcp.LayerPayload()
		s.packets++
		// fmt.Printf("- packet %d = %d bytes\n", s.packets, len(s.b))
	}
	written := copy(p, s.b)
	s.b = s.b[written:]
	return written, nil
}

func (s *stream) Info() packetInfo {
	return s.info
}

func (s *stream) Packets() int {
	return s.packets
}
