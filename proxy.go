package realtime

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

type Proxy struct {
	APIToken string
	WSSURL   string
	Upgrader websocket.Upgrader
}

type Message struct {
	Type int         `json:"type"`
	Data interface{} `json:"data"`
}

func NewProxy(apiToken string, wssURL string) *Proxy {
	return &Proxy{
		APIToken: apiToken,
		WSSURL:   wssURL,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (p *Proxy) Handle(w http.ResponseWriter, r *http.Request) {
	clientConn, err := p.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Failed to upgrade the connection to WebSocket connection!")
		return
	}
	defer clientConn.Close()

	openaiConn, res, err := p.connectOpenAI()
	if err != nil {
		log.Println("Failed to connect to OpenAI: ", res, err)
		return
	}
	defer openaiConn.Close()

	// starting bidirectional communication
	done := make(chan struct{})

	// client -> openai
	go p.pipeMessages(clientConn, openaiConn, done)

	// openai -> client
	go p.pipeMessages(openaiConn, clientConn, done)

	<-done
	log.Println("Bidirectional communication ended.")
}

func (p *Proxy) connectOpenAI() (*websocket.Conn, *http.Response, error) {
	u, err := url.Parse(p.WSSURL)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse the WSS API!")
	}

	headers := http.Header{
		"Authorization": {"Bearer " + p.APIToken},
		"User-Agent":    {"go-realtime"},
		"OpenAI-Beta":   {"realtime=v1"},
	}

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		return nil, resp, fmt.Errorf("failed to dial OpenAI: %w", err)
	}

	return conn, resp, nil
}

func (p *Proxy) pipeMessages(src, dst *websocket.Conn, done chan struct{}) {
	defer func() {
		src.Close()
		dst.Close()
		select {
		case <-done:
		default:
			close(done)
		}
	}()

	for {
		msgType, msg, err := src.ReadMessage()
		if err != nil {
			return
		}

		if err := dst.WriteMessage(msgType, msg); err != nil {
			return
		}
	}
}
