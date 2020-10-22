package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"os/signal"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	wg "github.com/sirmackk/wiregate"
)

type ServerConfig struct {
	iface           string
	wgIface         string
	wgPort          int
	wgCIDR          string
	mdnsServiceDesc string
	httpPort        int
	vpnPassword     string
	purgeInterval   int
}

func generateTLSCertKeyFiles(ifaceIP *net.IP) (string, string) {
	// Based on https://golang.org/src/crypto/tls/generate_cert.go
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Errorf("Error while generating private RSA key: %s", err)
		os.Exit(1)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Errorf("Error while generating serial number of x509 certificate: %s", err)
		os.Exit(1)
	}
	certTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"WireGate"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	certTemplate.IPAddresses = append(certTemplate.IPAddresses, *ifaceIP)
	certBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, &privKey.PublicKey, privKey)
	if err != nil {
		log.Errorf("Error while generating x509 certificate: %s", err)
		os.Exit(1)
	}
	certOut, err := ioutil.TempFile("", "WireGateCert.pem")
	if err != nil {
		log.Errorf("Error while creating temporary .pem cert file: %s", err)
		os.Exit(1)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		log.Errorf("Error while writing pem cert to temporary file: %s", err)
	}
	tmpDir, err := ioutil.TempDir("", "WireGatePemKey")
	if err != nil {
		log.Errorf("Error while creating temporary directory to store certificate key: %s", err)
		os.Exit(1)
	}
	keyPath := fmt.Sprintf("%s/%s", tmpDir, "key.pem")
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Errorf("Error while creating key.pem file in %s: %s", tmpDir, err)
		os.Exit(1)
	}
	defer keyOut.Close()
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		log.Errorf("Error while marshalling private key before writing to file: %s", err)
		os.Exit(1)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}); err != nil {
		log.Errorf("Error while writing key.pem data to disk: %s", err)
		os.Exit(1)
	}
	return certOut.Name(), keyOut.Name()
}

func server_main(conf *ServerConfig) {
	ipgen, err := wg.NewSimpleIPGen(conf.wgCIDR)
	if err != nil {
		log.Errorf("Error generating WireGuard subnet: %s", err)
		os.Exit(1)
	}
	log.Debugf("Setup SimpleIPGen with %d IPs in %s", len(ipgen.AvailableIPs), ipgen.BaseIPCIDR)
	log.Info("Generating WireGuard pub/priv key pair")

	wgPrivateKey, wgPublicKey := generateWGKeypair()
	log.Infof("Generated public WireGuard key %s", wgPublicKey)
	wgPrivateKeyPath, err := WriteRestrictedFile("WireGatePrivateKey", wgPrivateKey)
	if err != nil {
		log.Errorf("Error while writing private WireGuard key: %s", err)
		os.Exit(1)
	}
	log.Infof("Generated private WireGuard key and saved to %s", wgPrivateKeyPath)
	wgctrl, err := wg.NewShellWireguardControl(ipgen.BaseIP, ipgen.SubnetIP, ipgen.CIDR, strconv.Itoa(conf.wgPort), conf.wgIface, conf.iface, wgPrivateKeyPath)
	if err != nil {
		log.Errorf("Error while creating WireGate controller : %s", err)
		os.Exit(1)
	}
	err = wgctrl.CreateInterface()
	if err != nil {
		log.Errorf("Error while creating WireGuard interface: %s", err)
		os.Exit(1)
	}
	err = wgctrl.AddInterfaceRoute()
	if err != nil {
		log.Errorf("Error while adding WireGuard interface route: %s", err)
		os.Exit(1)
	}
	log.Infof("Created WireGuard interface %s, bridged to %s, and started WireGuard server on %s", conf.wgIface, conf.iface, wgctrl.EndpointIPPortPair)
	registry := wg.NewRegistry(ipgen, wgctrl)
	ifaceIP := net.ParseIP(wgctrl.EndpointIP)
	mdnsServer := wg.NewMDNSServer(conf.mdnsServiceDesc, &ifaceIP, conf.httpPort)
	httpAPI := &wg.HttpApi{
		Registry:           registry,
		EndpointIPPortPair: wgctrl.EndpointIPPortPair,
		VPNPassword:        conf.vpnPassword,
		WGServerPublicKey:  wgPublicKey,
		WGServerPeerIP:     ipgen.BaseIP,
	}

	httpCertPath, httpKeyPath := generateTLSCertKeyFiles(&ifaceIP)
	log.Infof("Generated TLS cert at %s and key at %s", httpCertPath, httpKeyPath)

	// define cleanup function to use from now on.
	cleanup := func() {
		log.Info("Stopping http api")
		err := httpAPI.Stop()
		if err != nil {
			log.Errorf("Error while stopping WireGate HTTP Control: %s", err)
		}
		log.Info("Stopping mdns server")
		mdnsServer.Stop()
		log.Info("Destroying interface")
		err = wgctrl.DestroyInterface()
		if err != nil {
			log.Errorf("Error while destroying WireGuard interface: %s", err)
		}
		os.Exit(0)
	}

	httpRunning := make(chan struct{})
	log.Info("Starting TLS HTTP Server...")
	// TODO: check if HTTP server came up alright
	httpAPI.Start(conf.httpPort, httpCertPath, httpKeyPath, httpRunning)
	log.Infof("Starting MDNS server...")
	err = mdnsServer.Start()
	if err != nil {
		cleanup()
		log.Errorf("Failed to start mdns server: %s", err)
		os.Exit(1)
	}

	termination := make(chan os.Signal, 1)
	signal.Notify(termination, os.Interrupt)
	go func() {
		s := <-termination
		log.Infof("\nReceived signal %s, terminating...\n", s)
		cleanup()
	}()

	log.Info("Starting registry purger")
	registry.StartPurging(conf.purgeInterval, conf.purgeInterval)

	<-httpRunning
}
