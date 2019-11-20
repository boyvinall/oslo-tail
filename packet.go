package main

import (
	"fmt"
	"log"
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type flow struct {
	Valid   bool
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint32
	DstPort uint32
}

func (f flow) String() string {
	if !f.Valid {
		return ""
	}
	return fmt.Sprintf("%s:%d-%s:%d", f.SrcIP, f.SrcPort, f.DstIP, f.DstPort)
}

func GetPacketFlow(packet gopacket.Packet) flow {

	if packet.NetworkLayer() == nil || packet.TransportLayer() == nil || packet.TransportLayer().LayerType() != layers.LayerTypeTCP {
		log.Println("Unusable packet")
		return flow{}
	}

	// t := packet.Metadata().Timestamp
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return flow{}
	}
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return flow{}
	}

	ip, _ := ipLayer.(*layers.IPv4)
	tcp, _ := tcpLayer.(*layers.TCP)
	return flow{
		Valid:   true,
		SrcIP:   ip.SrcIP,
		DstIP:   ip.DstIP,
		SrcPort: uint32(tcp.SrcPort),
		DstPort: uint32(tcp.DstPort),
	}
}
