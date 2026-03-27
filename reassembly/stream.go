package reassembly

import (
	"bufio"
	"io"
	"net/http"

	"github.com/google/gopacket"
	"github.com/google/gopacket/tcpassembly"
	"github.com/google/gopacket/tcpassembly/tcpreader"

	"httpinspector/models"
)

type HTTPStreamFactory struct {
	Transactions chan<- models.HTTPTransaction
}

func (f *HTTPStreamFactory) New(net, transport gopacket.Flow) tcpassembly.Stream {
	stream := tcpreader.NewReaderStream()
	go f.parseHTTP(net, transport, &stream)
	return &stream
}

func (f *HTTPStreamFactory) parseHTTP(net, transport gopacket.Flow, stream *tcpreader.ReaderStream) {
	buf := bufio.NewReader(stream)
	for {
		req, err := http.ReadRequest(buf)
		if err == io.EOF || err != nil {
			return
		}

		tx := models.HTTPTransaction{
			ID:       net.Src().String() + ":" + transport.Src().String(),
			Method:   req.Method,
			Host:     req.Host,
			Path:     req.URL.Path,
			SourceIP: net.Src().String(),
			DestIP:   net.Dst().String(),
		}

		f.Transactions <- tx
		req.Body.Close()
	}
}