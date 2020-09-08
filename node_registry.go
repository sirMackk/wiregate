package main

import (
	"fmt"
	"time"
)

type WgController interface {
	AddHost(string, string) error
	RemoveHost(string) error
}

type Node struct {
	PubKey, VPNIP string
	lastAliveAt   int64
}

func (n *Node) Beat() {
	n.lastAliveAt = time.Now().Unix()
}

type Registry struct {
	nodes     map[string]*Node
	IPGen     IPGenerator
	WgControl WgController
	purging   chan bool
}

func NewRegistry(ipgen IPGenerator, control WgController) *Registry {
	return &Registry{
		nodes:     make(map[string]*Node),
		IPGen:     ipgen,
		WgControl: control,
		purging:   make(chan bool),
	}
}

func (r *Registry) Get(publicKey string) (*Node, error) {
	if n, ok := r.nodes[publicKey]; ok {
		return n, nil
	}
	return nil, fmt.Errorf("Node with pubkey %s not found!", publicKey)
}

func (r *Registry) Put(publicKey string) (*Node, error) {
	if _, ok := r.nodes[publicKey]; ok {
		return nil, fmt.Errorf("Node with pubkey %s already exists", publicKey)
	}
	ip, err := r.IPGen.LeaseIP()
	if err != nil {
		return nil, fmt.Errorf("Problem assigning wg ip: %s", err)
	}
	n := Node{
		PubKey: publicKey,
		VPNIP:  ip,
	}
	n.Beat()
	err = r.WgControl.AddHost(publicKey, ip)
	if err != nil {
		return nil, fmt.Errorf("Problem with WgControl: %s", err)
	}
	r.nodes[publicKey] = &n

	return &n, nil
}

func (r *Registry) Delete(publicKey string) error {
	if _, ok := r.nodes[publicKey]; !ok {
		return fmt.Errorf("Node with pubkey %s not found!", publicKey)
	}
	err := r.WgControl.RemoveHost(publicKey)
	if err != nil {
		return fmt.Errorf("Problem with WgControl: %s", err)
	}
	ip := r.nodes[publicKey].VPNIP
	r.IPGen.ReleaseIP(ip)
	// TODO: can this leave in an incosistent state? eg system, registry, ipgen?
	delete(r.nodes, publicKey)
	return nil
}

func (r *Registry) GetRegisteredIPs() []string {
	registeredIPs := make([]string, 0, len(r.nodes))
	for _, n := range r.nodes {
		registeredIPs = append(registeredIPs, n.VPNIP)
	}
	return registeredIPs
}

func (r *Registry) StartPurging(deadline, interval int) {
	ticker := time.NewTicker(time.Duration(interval) * time.Second)

	go func() {
		defer ticker.Stop()
		for {
			expirationTime := time.Now().Unix() - int64(deadline)
			for key, node := range r.nodes {
				if node.lastAliveAt < expirationTime {
					fmt.Println("Deleting", key)
					r.Delete(key)
				}
			}
			select {
			case <-r.purging:
				return
			case <-ticker.C:
				continue
			}
		}
	}()
}

func (r *Registry) StopPurging() {
	r.purging <- true
}
