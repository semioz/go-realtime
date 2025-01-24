package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	realtime "github.com/semioz/go-realtime"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, falling back to system environment variables.")
	}

	apiToken := os.Getenv("OPENAI_API_KEY")
	if apiToken == "" {
		log.Fatal("OPENAI_API_KEY is not set in environment variables.")
	}

	proxy := realtime.NewProxy(
		apiToken,
		realtime.defaultWSSURL,
	)

	http.HandleFunc("/ws", proxy.Handle)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Server listening on :%s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
