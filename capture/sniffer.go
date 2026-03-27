package capture

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

func NewPacketSource(iface, filter string) (*gopacket.PacketSource, error) {
	handle, err := pcap.OpenLive(iface, 1600, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}

	if err := handle.SetBPFFilter(filter); err != nil {
		return nil, err
	}

	return gopacket.NewPacketSource(handle, handle.LinkType()), nil
}