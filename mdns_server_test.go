package wiregate

import (
	"net"
	"testing"

	"github.com/ideasynthesis/mdns"
)

func TestStartingServer(t *testing.T) {
	var called bool = false
	mockMDNSFunc := func(conf *mdns.Config) (*mdns.Server, error) {
		called = true
		return &mdns.Server{}, nil
	}
	NewServerFunc = mockMDNSFunc

	ip := net.IPv4(192, 168, 128, 10)
	serviceDesc := "serviceDescription"
	port := 9999
	server := NewMDNSServer(serviceDesc, &ip, port)

	err := server.Start()
	if err != nil {
		t.Errorf("Failed to start server: %v", err)
	}
	if !called {
		t.Errorf("Tried to start server, but 'called' is false")
	}
}
