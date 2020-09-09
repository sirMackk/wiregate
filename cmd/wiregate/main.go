package main

import (
	"flag"
	"fmt"
	"os"
	// logging
)

// TODO: help subcommand
// TODO: keep all IPs as net.IP structs instead of strings

func main() {
	var server = flag.NewFlagSet("server", flag.ExitOnError)
	var iface = server.String("interface", "", "REQUIRED: Network interface to use")
	var wgIface = server.String("wg-interface", "wg0", "Name for WireGuard interface")
	var wgPort = server.Int("wg-port", 51820, "WireGuard port")
	var wgCIDR = server.String("wg-cidr", "10.24.1.1/24", "IPv4 CIDR subnet for WireGuard VPN. The WireGuard interface will use the first subnet address")
	var mdnsServiceDesc = server.String("http-service-description", "", "MDNS WireGate HTTP Control description")
	var httpPort = server.Int("http-port", 38490, "WireGate HTTP Control port")
	var vpnPassword = server.String("vpn-password", "", "REQUIRED: Password to register with the WireGate VPN")
	var purgeInterval = server.Int("purge-interval", 10, "Interval to purge unresponsive clients")

	// TODO clean up client flags
	var client = flag.NewFlagSet("client", flag.ExitOnError)
	var version = client.Bool("version", false, "Client version")

	if len(os.Args) < 2 {
		fmt.Println("Expected 'server' or 'client' subcommands")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "server":
		if err := server.Parse(os.Args[2:]); err == nil {
			if *iface == "" {
				fmt.Printf("Missing '-interface' argument!")
				os.Exit(1)
			}
			if *vpnPassword == "" {
				// TODO: check for weak password; generate share-able password if empty.
				fmt.Printf("Missing '-vpn-password' argment!")
				os.Exit(1)
			}
			conf := &ServerConfig{
				iface:           *iface,
				wgIface:         *wgIface,
				wgPort:          *wgPort,
				wgCIDR:          *wgCIDR,
				mdnsServiceDesc: *mdnsServiceDesc,
				httpPort:        *httpPort,
				vpnPassword:     *vpnPassword,
				purgeInterval:   *purgeInterval,
			}
			server_main(conf)
		}
	case "client":
		if err := client.Parse(os.Args[2:]); err == nil {
			if *version {
				fmt.Printf("0\n")
				os.Exit(0)
			}
			client_main()
		}
	default:
		fmt.Printf("Unknown subcommand '%s'\n", os.Args[1])
		os.Exit(1)
	}
}
