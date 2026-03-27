package main

import (
	"log"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/tcpassembly"

	"httpinspector/capture"
	"httpinspector/models"
	"httpinspector/reassembly"
	"httpinspector/ui"
)

func getActiveInterface() string {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		log.Fatal(err)
	}

	for _, device := range devices {
		for _, address := range device.Addresses {
			if address.IP.To4() != nil && !address.IP.IsLoopback() {
				return device.Name
			}
		}
	}

	log.Fatal("No active IPv4 network interface found")
	return ""
}

func main() {
	iface := getActiveInterface()
	filter := "tcp and port 80"

	txChan := make(chan models.HTTPTransaction, 100)

	packetSource, err := capture.NewPacketSource(iface, filter)
	if err != nil {
		log.Fatal(err)
	}

	streamFactory := &reassembly.HTTPStreamFactory{Transactions: txChan}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case packet := <-packetSource.Packets():
				if packet == nil {
					return
				}
				if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
					tcp, _ := tcpLayer.(*layers.TCP)
					assembler.AssembleWithTimestamp(
						packet.NetworkLayer().NetworkFlow(),
						tcp,
						packet.Metadata().Timestamp,
					)
				}
			case <-ticker.C:
				assembler.FlushOlderThan(time.Now().Add(-time.Minute * 2))
			}
		}
	}()

	p := tea.NewProgram(ui.New(txChan))
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}