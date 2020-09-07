package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type HttpApi struct {
	registry *Registry
	// this should be a shared config struct
	endpoint string
}

type RegistrationRequest struct {
	PublicKey string
}

type RegistrationReply struct {
	NodeIp     string
	Endpoint   string
	AllowedIps []string
}

type DeregistrationRequest struct {
	PublicKey string
}

type HeartBeatRequest struct {
	PublicKey string
}

type HeartBeatResponse struct {
	AllowedIps []string
}

func (h *HttpApi) registerNode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	decoder := json.NewDecoder(req.Body)
	var r RegistrationRequest
	err := decoder.Decode(&r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
	n, err := h.registry.Put(r.PublicKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := &RegistrationReply{
		NodeIp:     n.VPNIP,
		Endpoint:   h.endpoint,
		AllowedIps: h.registry.GetRegisteredIPs(),
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
}

func (h *HttpApi) removeNode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	decoder := json.NewDecoder(req.Body)
	var r DeregistrationRequest
	err := decoder.Decode(&r)
	if err != nil {
		http.Error(w, "Error while decoding json", http.StatusInternalServerError)
		return
	}
	if err := h.registry.Delete(r.PublicKey); err != nil {
		http.Error(w, "Node not found", http.StatusNotFound)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HttpApi) heartBeat(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	decoder := json.NewDecoder(req.Body)
	var hb HeartBeatRequest
	err := decoder.Decode(&hb)
	if err != nil {
		http.Error(w, "Error while decoding json", http.StatusInternalServerError)
		return
	}
	n, err := h.registry.Get(hb.PublicKey)
	if err != nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	n.Beat()

	response := &HeartBeatResponse{
		AllowedIps: h.registry.GetRegisteredIPs(),
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
}
