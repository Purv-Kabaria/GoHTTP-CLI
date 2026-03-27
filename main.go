package main

import (
	"fmt"
	"log"
	"os"
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
	logFile, err := os.OpenFile("inspector.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	fmt.Println("Initializing Network Inspector...")
	iface := getActiveInterface()
	filter := "tcp and (port 80 or port 443)"

	fmt.Printf("\nBinding to interface: %s\n", iface)
	fmt.Printf("BPF Filter active: %s\n", filter)
	fmt.Println("\nStarting UI in 3 seconds...")
	time.Sleep(3 * time.Second)

	txChan := make(chan models.HTTPTransaction, 100)

	packetSource, err := capture.NewPacketSource(iface, filter)
	if err != nil {
		log.Fatal(err)
	}

	tracker := reassembly.NewTransactionTracker()
	streamFactory := &reassembly.HTTPStreamFactory{
		Transactions: txChan,
		Tracker:      tracker,
	}
	streamPool := tcpassembly.NewStreamPool(streamFactory)
	assembler := tcpassembly.NewAssembler(streamPool)

	go func() {
		ticker := time.NewTicker(time.Second * 2)
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
				assembler.FlushOlderThan(time.Now().Add(-time.Second * 5))
			}
		}
	}()

	p := tea.NewProgram(ui.New(txChan))
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}