package main

import (
	"fmt"
	"os/exec"
	"time"
)

var execCommand = NewCommand

type Commander interface {
	CombinedOutput() ([]byte, error)
}

type Command struct {
	Cmd *exec.Cmd
}

func NewCommand(cmd string, args ...string) Commander {
	return Command{
		Cmd: exec.Command(cmd, args...),
	}
}

func (c Command) CombinedOutput() ([]byte, error) {
	return c.Cmd.CombinedOutput()
}

type ShellWireguardControl struct {
	InterfaceAddress string
	ListenPort       string
	InterfaceName    string
	PrivateKeyPath   string
	PostUp           string
	PostDown         string
}

func NewShellWireguardControl(address, listenPort, interfaceName, privateKeypath, postUp, postDown string) *ShellWireguardControl {
	s := &ShellWireguardControl{
		PrivateKeyPath:   privateKeypath,
		InterfaceAddress: address,
		ListenPort:       listenPort,
		InterfaceName:    interfaceName,
		PostUp:           postUp,
		PostDown:         postDown,
	}
	return s
}

func (s *ShellWireguardControl) CreateInterface() error {
	//TODO figure out getting ipv4/ipv6 proto version
	proto := "-4"

	if _, err := exec.LookPath("ip"); err != nil {
		return fmt.Errorf("Command 'ip' not found!")
	}
	if _, err := exec.LookPath("wg"); err != nil {
		return fmt.Errorf("Command 'wg' not found!")
	}
	// create interface
	createIface := execCommand("ip", "link", "add", "dev", s.InterfaceName, "type", "wireguard")
	if out, err := createIface.CombinedOutput(); err != nil {
		return fmt.Errorf("Creating interface failed when calling ip: %s\n%s", err, out)
	}

	// add ip addr to iface
	ipSetAddr := execCommand("ip", proto, "address", "add", s.InterfaceAddress, "dev", s.InterfaceName)
	if out, err := ipSetAddr.CombinedOutput(); err != nil {
		return fmt.Errorf("Adding ip address to interface %s failed: %s\n%s", s.InterfaceName, err, out)
	}

	// wg setconf flags and args
	wgSetConfig := execCommand("wg", "set", s.InterfaceName, "listen-port", s.ListenPort, "private-key", s.PrivateKeyPath)
	if out, err := wgSetConfig.CombinedOutput(); err != nil {
		return fmt.Errorf("Creating interface failed when calling wg: %s\n%s", err, out)
	}

	// activate interface
	activateIface := execCommand("ip", "link", "set", "up", "dev", s.InterfaceName)
	if out, err := activateIface.CombinedOutput(); err != nil {
		return fmt.Errorf("Activating interface failed when calling ip: %s\n%s", err, out)
	}

	// execute postup
	postUpCmd := execCommand("sh", "-c", s.PostUp)
	if out, err := postUpCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Activating interface failed when executing post-up cmd: %s\n%s", err, out)
	}

	return nil
}

func (s *ShellWireguardControl) DestroyInterface() error {
	//TODO figure out getting ipv4/ipv6 proto version
	//proto := "-4"
	if _, err := exec.LookPath("ip"); err != nil {
		return fmt.Errorf("Command 'ip' not found!")
	}
	if _, err := exec.LookPath("wg"); err != nil {
		return fmt.Errorf("Command 'wg' not found!")
	}
	// ip -4 rule show - wg allowed-ips to see if these change ip -4 rule show results
	ipLinkDelete := execCommand("ip", "link", "delete", "dev", s.InterfaceName, "type", "wireguard")
	if out, err := ipLinkDelete.CombinedOutput(); err != nil {
		return fmt.Errorf("Deleting interface %s failed: %s\n%s", s.InterfaceName, err, out)
	}
	postDownCmd := execCommand("sh", "-c", s.PostDown)
	if out, err := postDownCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("Executing post-down cmd failed: %s\n%s", err, out)
	}

	return nil
}

func (s *ShellWireguardControl) AddHost(pubkey, peerIP string) error {
	wgSetPeer := execCommand("wg", "set", s.InterfaceName, "peer", pubkey, "allowed-ips", peerIP)
	if out, err := wgSetPeer.CombinedOutput(); err != nil {
		return fmt.Errorf("Failed to add peer (%s - %s): %s\n%s", pubkey, peerIP, err, out)
	}
	return nil
}

func (s *ShellWireguardControl) RemoveHost(pubkey string) error {
	wgSetPeerRemove := execCommand("wg", "set", s.InterfaceName, "peer", pubkey, "remove")
	if out, err := wgSetPeerRemove.CombinedOutput(); err != nil {
		return fmt.Errorf("Failed to removed peer (%s): %s\n%s", pubkey, err, out)
	}
	return nil
}

func main() {
	s := &ShellWireguardControl{
		PrivateKeyPath:   "/etc/wireguard/wg0/privatekey",
		InterfaceAddress: "10.24.1.1/24",
		ListenPort:       "51820",
		InterfaceName:    "wg0",
		PostUp:           "iptables -A FORWARD -i wg0 -j ACCEPT; iptables -A FORWARD -o wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE",
		PostDown:         "iptables -D FORWARD -i wg0 -j ACCEPT; iptables -D FORWARD -o wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE",
	}
	fmt.Println("starting wg0")
	if err := s.CreateInterface(); err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println("All good, adding peer")
	}
	err := s.AddHost("S+f+y/K7Sr23ptSjd3xTDC23gDYZjQGbWrk1LSk7z1I=", "10.24.1.10/32")
	if err != nil {
		fmt.Println("Problem adding host ", err)
	}
	fmt.Println("host added, sleeping 12")
	time.Sleep(120 * time.Second)
	fmt.Println("removing peer")
	if err == nil {
		err = s.RemoveHost("S+f+y/K7Sr23ptSjd3xTDC23gDYZjQGbWrk1LSk7z1I=")
		fmt.Println("removed host, sleeping 12")
		time.Sleep(12 * time.Second)
		if err != nil {
			fmt.Println("Problem removing host ", err)
		}
	}

	if err := s.DestroyInterface(); err != nil {
		fmt.Println(err)
		return
	} else {
		fmt.Println("Destroyed good")
	}
}
