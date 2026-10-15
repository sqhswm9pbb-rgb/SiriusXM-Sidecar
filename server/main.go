package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	sidecar "github.com/codester28351/siriusxm-sidecar"
)

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func detectLANIP() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP == nil {
				continue
			}
			ip := ipNet.IP
			if ip.To4() != nil && !ip.IsLoopback() {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

func main() {
	host := envOr("SIDECAR_HOST", "0.0.0.0")
	port := envOr("SIDECAR_PORT", "8091")
	lanIP := envOr("SIDECAR_LAN_IP", detectLANIP())
	baseURL := envOr("SIDECAR_BASE_URL", "http://"+lanIP+":"+port)

	if err := sidecar.ValidateBaseURL(baseURL); err != nil {
		log.Fatal(err)
	}

	if len(os.Args) != 3 {
		log.Fatalf("usage: %s USERNAME PASSWORD", os.Args[0])
	}

	provider := sidecar.NewScriptProvider(os.Args[1], os.Args[2])
	if err := provider.Start(context.Background()); err != nil {
		log.Fatal(err)
	}
	defer provider.Stop()

	api := sidecar.NewAPI(provider, baseURL)
	addr := host + ":" + port

	log.Printf("SiriusXM sidecar listening on %s", addr)
	log.Printf("LAN base URL: %s", baseURL)
	log.Printf("Station list: %s/api/stations", baseURL)
	log.Fatal(http.ListenAndServe(addr, api.Handler()))
}
