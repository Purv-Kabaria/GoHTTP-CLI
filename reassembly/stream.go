package reassembly

import (
	"bufio"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"

	"httpinspector/models"
)

type HTTPStreamFactory struct {
	Transactions chan<- models.HTTPTransaction
	Tracker      *TransactionTracker
}

func (f *HTTPStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	stream := tcpreader.NewReaderStream()
	go f.parseStream(net, transport, &stream)
	return &stream
}

func (f *HTTPStreamFactory) parseStream(net, transport gopacket.Flow, stream *tcpreader.ReaderStream) {
	buf := bufio.NewReader(stream)
	srcPort := transport.Src().String()
	dstPort := transport.Dst().String()

	if dstPort == "80" {
		f.handleRequest(net, transport, buf)
	} else if srcPort == "80" {
		f.handleResponse(net, transport, buf)
	}
}

func (f *HTTPStreamFactory) handleRequest(net, transport gopacket.Flow, buf *bufio.Reader) {
	connID := net.Src().String() + ":" + transport.Src().String() + "-" + net.Dst().String() + ":" + transport.Dst().String()

	for {
		req, err := http.ReadRequest(buf)
		if err == io.EOF {
			return
		} else if err != nil {
			log.Printf("Req Parse Error [%s]: %v\n", connID, err)
			return
		}

		tx := models.HTTPTransaction{
			ID:          connID,
			Method:      req.Method,
			Host:        req.Host,
			Path:        req.URL.Path,
			RequestTime: time.Now(),
			SourceIP:    net.Src().String(),
			SourcePort:  transport.Src().String(),
			DestIP:      net.Dst().String(),
			DestPort:    transport.Dst().String(),
		}

		io.Copy(io.Discard, req.Body)
		req.Body.Close()

		if completedTx, ok := f.Tracker.AddRequest(connID, tx); ok {
			f.Transactions <- *completedTx
		}
	}
}

func (f *HTTPStreamFactory) handleResponse(net, transport gopacket.Flow, buf *bufio.Reader) {
	connID := net.Dst().String() + ":" + transport.Dst().String() + "-" + net.Src().String() + ":" + transport.Src().String()

	for {
		res, err := http.ReadResponse(buf, nil)
		if err == io.EOF {
			return
		} else if err != nil {
			log.Printf("Res Parse Error [%s]: %v\n", connID, err)
			return
		}

		tx := models.HTTPTransaction{
			StatusCode:    res.StatusCode,
			ContentLength: res.ContentLength,
		}
		
		responseTime := time.Now()

		io.Copy(io.Discard, res.Body)
		res.Body.Close()

		if completedTx, ok := f.Tracker.AddResponse(connID, tx, responseTime); ok {
			f.Transactions <- *completedTx
		}
	}
}