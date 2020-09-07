package main

import (
	"fmt"
	"time"
)

type IPGenerator interface {
	NewIP() string
}

type Node struct {
	PubKey, VPNIP string
	lastAliveAt   int64
}

func (n *Node) Beat() {
	n.lastAliveAt = time.Now().Unix()
}

type Registry struct {
	nodes   map[string]*Node
	IPGen   IPGenerator
	purging chan bool
}

func NewRegistry(ipgen IPGenerator) *Registry {
	return &Registry{
		nodes:   make(map[string]*Node),
		IPGen:   ipgen,
		purging: make(chan bool),
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
	ip := r.IPGen.NewIP()
	n := Node{
		PubKey: publicKey,
		VPNIP:  ip,
	}
	n.Beat()
	r.nodes[publicKey] = &n

	return &n, nil
}

func (r *Registry) Delete(publicKey string) error {
	if _, ok := r.nodes[publicKey]; !ok {
		return fmt.Errorf("Node with pubkey %s not found!", publicKey)
	}
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
	expirationTime := int64(deadline)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-r.purging:
				return
			case now := <-ticker.C:
				for key, node := range r.nodes {
					if node.lastAliveAt < now.Unix()-expirationTime {
						delete(r.nodes, key)
					}
				}
			}
		}
	}()
}

func (r *Registry) StopPurging() {
	r.purging <- true
}
