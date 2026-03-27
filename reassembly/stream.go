package reassembly

import (
	"bufio"
	"io"
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
		f.handleHTTPRequest(net, transport, buf)
	} else if srcPort == "80" {
		f.handleHTTPResponse(net, transport, buf)
	} else if dstPort == "443" {
		f.handleTLSClientHello(net, transport, buf)
	} else if srcPort == "443" {
		f.handleTLSServerHello(net, transport, buf)
	}
}

func readBody(r io.ReadCloser) []byte {
	if r == nil {
		return nil
	}
	defer r.Close()
	lr := io.LimitReader(r, 8192)
	b, _ := io.ReadAll(lr)
	io.Copy(io.Discard, r)
	return b
}

func (f *HTTPStreamFactory) handleHTTPRequest(net, transport gopacket.Flow, buf *bufio.Reader) {
	connID := net.Src().String() + ":" + transport.Src().String() + "-" + net.Dst().String() + ":" + transport.Dst().String()

	for {
		req, err := http.ReadRequest(buf)
		if err == io.EOF {
			return
		} else if err != nil {
			return
		}

		tx := models.HTTPTransaction{
			ID:          connID,
			Protocol:    "HTTP",
			Method:      req.Method,
			Host:        req.Host,
			Path:        req.URL.Path,
			RequestTime: time.Now(),
			SourceIP:    net.Src().String(),
			SourcePort:  transport.Src().String(),
			DestIP:      net.Dst().String(),
			DestPort:    transport.Dst().String(),
			ReqHeaders:  req.Header.Clone(),
			ReqBody:     readBody(req.Body),
		}

		if completedTx, ok := f.Tracker.AddRequest(connID, tx); ok {
			f.Transactions <- *completedTx
		}
	}
}

func (f *HTTPStreamFactory) handleHTTPResponse(net, transport gopacket.Flow, buf *bufio.Reader) {
	connID := net.Dst().String() + ":" + transport.Dst().String() + "-" + net.Src().String() + ":" + transport.Src().String()

	for {
		res, err := http.ReadResponse(buf, nil)
		if err == io.EOF {
			return
		} else if err != nil {
			return
		}

		tx := models.HTTPTransaction{
			StatusCode:    res.StatusCode,
			ContentLength: res.ContentLength,
			ResHeaders:    res.Header.Clone(),
			ResBody:       readBody(res.Body),
		}

		responseTime := time.Now()

		if completedTx, ok := f.Tracker.AddResponse(connID, tx, responseTime); ok {
			f.Transactions <- *completedTx
		}
	}
}

func (f *HTTPStreamFactory) handleTLSClientHello(net, transport gopacket.Flow, buf *bufio.Reader) {
	connID := net.Src().String() + ":" + transport.Src().String() + "-" + net.Dst().String() + ":" + transport.Dst().String()

	sni, err := ExtractSNI(buf)
	if err != nil {
		io.Copy(io.Discard, buf)
		return
	}

	tx := models.HTTPTransaction{
		ID:          connID,
		Protocol:    "HTTPS",
		Method:      "HTTPS",
		Host:        sni,
		Path:        "/* Encrypted */",
		RequestTime: time.Now(),
		SourceIP:    net.Src().String(),
		SourcePort:  transport.Src().String(),
		DestIP:      net.Dst().String(),
		DestPort:    transport.Dst().String(),
	}

	if completedTx, ok := f.Tracker.AddRequest(connID, tx); ok {
		f.Transactions <- *completedTx
	}
	
	io.Copy(io.Discard, buf)
}

func (f *HTTPStreamFactory) handleTLSServerHello(net, transport gopacket.Flow, buf *bufio.Reader) {
	connID := net.Dst().String() + ":" + transport.Dst().String() + "-" + net.Src().String() + ":" + transport.Src().String()

	tx := models.HTTPTransaction{
		StatusCode: 0, 
	}

	responseTime := time.Now()

	if completedTx, ok := f.Tracker.AddResponse(connID, tx, responseTime); ok {
		f.Transactions <- *completedTx
	}

	io.Copy(io.Discard, buf)
}