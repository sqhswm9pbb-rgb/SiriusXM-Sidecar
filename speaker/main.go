package main

import (
	"log"
	"net/http"
	"os"

	sidecar "github.com/codester28351/siriusxm-sidecar"
)

func main() {
	host := os.Getenv("SIDECAR_HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("SIDECAR_PORT")
	if port == "" {
		port = "8091"
	}
	baseURL := os.Getenv("SIDECAR_BASE_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:" + port
	}

	provider := sidecar.NewDemoProvider()
	api := sidecar.NewAPI(provider, baseURL)
	addr := host + ":" + port

	log.Printf("Speaker-local test sidecar listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, api.Handler()))
}
