package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

type FakeIPGen struct {
	count int
}

func (f *FakeIPGen) NewIP() string {
	f.count += 1
	return fmt.Sprintf("1.1.1.%d", f.count)
}

func TestGettingNode(t *testing.T) {
	pubkey := "publicKey1"
	registry := NewRegistry(&FakeIPGen{})
	n1, err := registry.Put(pubkey)
	if err != nil {
		t.Errorf("Problem with creating registry entry: %v", err)
	}

	_, err = registry.Put(pubkey)
	if err == nil {
		t.Errorf("Created node with duplicated pubkey!")
	}

	n2, err := registry.Get(pubkey)
	if !reflect.DeepEqual(n1, n2) {
		t.Errorf("Get didnt return the same node as was Put: %+v != %+v", n1, n2)
	}

	noNode, err := registry.Get("pubkey_doesnt_exist")
	if noNode != nil || err == nil {
		t.Errorf("Tried to Get inexistent key and didnt fail: %v", noNode)
	}
}

func TestDeletingNode(t *testing.T) {
	pubkey := "publicKey1"
	registry := NewRegistry(&FakeIPGen{})
	registry.Put(pubkey)
	err := registry.Delete(pubkey)
	if err != nil {
		t.Errorf("Problem with deleting node: %v", err)
	}

	noNode, err := registry.Get(pubkey)
	if noNode != nil || err == nil {
		t.Errorf("Failed to delete node!")
	}

	err = registry.Delete(pubkey)
	if err == nil {
		t.Errorf("Attempted to delete inexistent node and succeeded!?")
	}
}

func TestGetRegisteredIPs(t *testing.T) {
	pubkey1 := "publicKey1"
	pubkey2 := "publicKey2"
	registry := NewRegistry(&FakeIPGen{})

	n1, _ := registry.Put(pubkey1)
	n2, _ := registry.Put(pubkey2)
	expectedIPs := []string{n1.VPNIP, n2.VPNIP}
	resultingIPs := registry.GetRegisteredIPs()
	if len(expectedIPs) != len(resultingIPs) {
		t.Errorf("Was expecting %d IPs, but got %d!", len(expectedIPs), len(resultingIPs))
	}
	if !reflect.DeepEqual(resultingIPs, expectedIPs) {
		t.Errorf("IPs do not match! Expected %v, got %v", expectedIPs, resultingIPs)
	}
}

func TestPurgingGoro(t *testing.T) {
	pubkey1 := "publicKey1"
	pubkey2 := "publicKey2"
	registry := NewRegistry(&FakeIPGen{})

	n1, _ := registry.Put(pubkey1)
	n1.lastAliveAt -= 1
	n2, _ := registry.Put(pubkey2)
	n2.lastAliveAt += 10

	registry.StartPurging(1, 1)
	time.Sleep(1 * time.Second)
	registry.StopPurging()

	noNode, err := registry.Get(pubkey1)
	if noNode != nil || err == nil {
		t.Errorf("Expected node '%s' to be purged, but it wasnt", pubkey1)
	}
	n2Node, err := registry.Get(pubkey2)
	if n2Node == nil || !reflect.DeepEqual(n2, n2Node) {
		t.Errorf("Expected node 2 '%s' to exist, but got %v", pubkey2, n2Node)
	}
}
