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
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"time"

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

func generateWGPrivateKey() string {
	if _, err := exec.LookPath("wg"); err != nil {
		fmt.Println("Command 'ip' not found!")
		os.Exit(1)
	}
	genKey := exec.Command("wg", "genkey")
	key, err := genKey.CombinedOutput()
	if err != nil {
		fmt.Printf("Error while generating private WireGuard key: %s\n", err)
		os.Exit(1)
	}
	return string(key)
}

func writePrivateKeyToFile(key string) string {
	keyFile, err := ioutil.TempFile("", "WireGatePrivateKey")
	if err != nil {
		fmt.Printf("Error while writing WireGuard private key to file: %s\n", err)
		os.Exit(1)
	}
	defer keyFile.Close()
	_, err = keyFile.WriteString(key)
	if err != nil {
		fmt.Printf("Error while writing WireGuard private key file to '%s': %s\n", keyFile.Name(), err)
		os.Exit(1)
	}
	return keyFile.Name()
}

func generateWGPublicKey(privateKey string) string {
	if _, err := exec.LookPath("wg"); err != nil {
		fmt.Println("Command 'ip' not found!")
		os.Exit(1)
	}
	wgPubKey := exec.Command("wg", "pubkey")
	wgPubKey.Stdin = strings.NewReader(privateKey)
	pubKey, err := wgPubKey.CombinedOutput()
	if err != nil {
		fmt.Printf("Error while generating WireGuard public key: %s\n", err)
		os.Exit(1)
	}
	return string(pubKey)
}

func generateTLSCertKeyFiles(ifaceIP *net.IP) (string, string) {
	// Based on https://golang.org/src/crypto/tls/generate_cert.go
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Error while generating private RSA key: %s\n", err)
		os.Exit(1)
	}
	notBefore := time.Now()
	notAfter := notBefore.Add(24 * time.Hour)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		fmt.Printf("Error while generating serial number of x509 certificate: %s\n", err)
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
	certBytes, err := x509.CreateCertificate(rand.Reader, &certTemplate, &certTemplate, privKey.PublicKey, privKey)
	if err != nil {
		fmt.Printf("Error while generating x509 certificate: %s\n", err)
		os.Exit(1)
	}
	certOut, err := ioutil.TempFile("", "WireGateCert.pem")
	if err != nil {
		fmt.Printf("Error while creating temporary .pem cert file: %s\n", err)
		os.Exit(1)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		fmt.Printf("Error while writing pem cert to temporary file: %s\n", err)
	}
	tmpDir, err := ioutil.TempDir("", "WireGatePemKey")
	if err != nil {
		fmt.Printf("Error while creating temporary directory to store certificate key: %s\n", err)
		os.Exit(1)
	}
	keyOut, err := os.OpenFile(fmt.Sprintf("%s/%s", tmpDir, "key.pem"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		fmt.Printf("Error while creating key.pem file in %s: %s\n", tmpDir, err)
		os.Exit(1)
	}
	defer keyOut.Close()
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		fmt.Printf("Error while marshalling private key before writing to file: %s\n", err)
		os.Exit(1)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRVIVATE KEY", Bytes: keyBytes}); err != nil {
		fmt.Printf("Error while writing key.pem data to disk: %s\n", err)
		os.Exit(1)
	}
	return certOut.Name(), keyOut.Name()
}

func server_main(conf *ServerConfig) {
	ipgen, err := wg.NewSimpleIPGen(conf.wgCIDR)
	if err != nil {
		fmt.Printf("Error generating WireGuard subnet: %s\n", err)
		os.Exit(1)
	}
	wgPrivateKey := generateWGPrivateKey()
	wgPrivateKeyPath := writePrivateKeyToFile(wgPrivateKey)
	wgPublicKey := generateWGPublicKey(wgPrivateKey)
	wgctrl, err := wg.NewShellWireguardControl(ipgen.BaseIP, strconv.Itoa(conf.wgPort), conf.wgIface, conf.iface, wgPrivateKeyPath)
	if err != nil {
		fmt.Printf("Error while creating WireGate controller : %s\n", err)
		os.Exit(1)
	}
	err = wgctrl.CreateInterface()
	if err != nil {
		fmt.Printf("Error while creating WireGuard interface: %s\n", err)
		os.Exit(1)
	}
	registry := wg.NewRegistry(ipgen, wgctrl)
	ifaceIP := net.ParseIP(wgctrl.EndpointIP)
	mdnsServer := wg.NewMDNSServer(conf.mdnsServiceDesc, &ifaceIP, conf.httpPort)
	httpAPI := &wg.HttpApi{
		Registry:           registry,
		EndpointIPPortPair: wgctrl.EndpointIPPortPair,
		VPNPassword:        conf.vpnPassword,
		WGServerPublicKey:  wgPublicKey,
	}

	cleanup := func() {
		err := httpAPI.Stop()
		if err != nil {
			fmt.Printf("Error while stopping WireGate HTTP Control: %s\n", err)
		}
		mdnsServer.Stop()
		err = wgctrl.DestroyInterface()
		if err != nil {
			fmt.Printf("Error while destroying WireGuard interface: %s\n", err)
		}
	}

	httpCertPath, httpKeyPath := generateTLSCertKeyFiles(&ifaceIP)
	httpRunning := make(chan struct{})
	httpAPI.Start(conf.httpPort, httpCertPath, httpKeyPath, httpRunning)
	err = mdnsServer.Start()
	if err != nil {
		cleanup()
		fmt.Printf("Failed to start mdns server: %s\n", err)
		os.Exit(1)
	}

	termination := make(chan os.Signal, 1)
	signal.Notify(termination, os.Interrupt)
	go func() {
		s := <-termination
		fmt.Printf("\nReceived signal %s, terminating...\n", s)
		cleanup()
	}()

	registry.StartPurging(conf.purgeInterval, conf.purgeInterval)

	<-httpRunning
}
