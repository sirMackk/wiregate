package wiregate

import (
	"net"
	"os"

	"github.com/ideasynthesis/mdns"
	log "github.com/sirupsen/logrus"
)

var NewServerFunc = mdns.NewServer

type MDNSServer struct {
	server      *mdns.Server
	hostname    string
	serviceName string
	description string

	ip      *net.IP
	port    int
	running bool
}

func NewMDNSServer(description string, ip *net.IP, port int) *MDNSServer {
	hostname, _ := os.Hostname()
	server := MDNSServer{
		hostname:    hostname,
		serviceName: "_wiregate._tcp",
		description: description,
		ip:          ip,
		port:        port,
		running:     false,
	}
	return &server
}

func (m *MDNSServer) Start() error {
	descriptions := []string{m.description}
	// domain == "", results in ".local"
	service, err := mdns.NewMDNSService(m.hostname, m.serviceName, "", "", m.port, []net.IP{*m.ip}, descriptions)
	if err != nil {
		return err
	}
	log.Debugf("Starting MDNSServer with config %#v", service)
	server, err := NewServerFunc(&mdns.Config{Zone: service})
	if err != nil {
		return err
	}
	m.server = server
	m.running = true
	return nil
}

func (m *MDNSServer) Stop() {
	if m.running {
		m.server.Shutdown()
	}
}
