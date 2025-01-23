package main

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

const realtimeWSS = "wss://api.openai.com/v1/realtime?model=gpt-4o-realtime-preview-2024-10-01"

type Proxy struct {
	APIToken string
}

type Message struct {
	Type int         `json:"type"`
	Data interface{} `json:"data"`
}

func NewProxy(apiToken string) *Proxy {
	return &Proxy{APIToken: apiToken}
}

func (p *Proxy) Handle(w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	clientConn, err := upgrader.Upgrade(w, r, nil)
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

	// clientConn -> openaiConn
	go func() {
		defer func() {
			// ensure cleanup happens here
			openaiConn.Close()
			clientConn.Close()
			log.Println("Connection closed.")

			select {
			case <-done:
			default:
				close(done)
			}
		}()

		for {
			messageType, msg, err := clientConn.ReadMessage()
			if err != nil {
				log.Println("Client read error:", err)
				return
			}
			if err := openaiConn.WriteMessage(messageType, msg); err != nil {
				log.Println("OpenAI send error:", err)
				return
			}
		}
	}()

	// openaiConn -> clientConn
	go func() {
		defer func() {
			openaiConn.Close()
			clientConn.Close()
			log.Println("Connection closed.")
			select {
			case <-done:
			default:
				close(done)
			}
		}()

		for {
			messageType, msg, err := openaiConn.ReadMessage()
			if err != nil {
				log.Println("OpenAI read error:", err)
				return
			}
			if err := clientConn.WriteMessage(messageType, msg); err != nil {
				log.Println("Client send error:", err)
				return
			}
		}
	}()

	<-done
	log.Println("Bidirectional communication ended.")
}

func (p *Proxy) connectOpenAI() (*websocket.Conn, *http.Response, error) {
	u, err := url.Parse(realtimeWSS)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse the WSS API!")
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+p.APIToken)
	headers.Set("User-Agent", "go-realtime")
	headers.Set("OpenAI-Beta", "realtime=v1")

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to establish WebSocket connection")
	}

	return conn, resp, nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, falling back to system environment variables.")
	}

	apiToken := os.Getenv("OPENAI_API_KEY")
	if apiToken == "" {
		log.Fatal("OPENAI_API_KEY is not set in environment variables.")
	}

	proxy := NewProxy(apiToken)

	http.HandleFunc("/ws", proxy.Handle)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Server listening on :%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
