package main

import (
	"fmt"
	"os"

	"github.com/ideasynthesis/mdns"
)

var NewServerFunc = mdns.NewServer

type MDNSServer struct {
	server *mdns.Server
	hostname string
	serviceName string
	description string
	port int
	running bool
}

func NewMDNSServer(serviceName, description string, port int) *MDNSServer {
	hostname, _ := os.Hostname()
	server := MDNSServer{
		hostname: fmt.Sprintf("%s.", hostname),
		serviceName: serviceName,
		description: description,
		port: port,
	}
	return &server
}

func (m *MDNSServer) Start() error {
	descriptions := []string{m.description}
	// domain == "", results in ".local"
	// TODO make sure ips == nil argument doesnt mean external DNS lookups
	service, err := mdns.NewMDNSService(m.hostname, m.serviceName, "", m.hostname, m.port, nil, descriptions)
	if err != nil {
		return err
	}
	server, err := NewServerFunc(&mdns.Config{Zone: service})
	m.server = server
	return err
}

func (m *MDNSServer) Stop() {
	if m.running {
		m.server.Shutdown()
	}
}
