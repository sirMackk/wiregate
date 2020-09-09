package wiregate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type HttpApi struct {
	server             *http.Server
	running            bool
	Registry           *Registry
	EndpointIPPortPair string
	VPNPassword        string
	WGServerPublicKey  string
}

type RegistrationRequest struct {
	PublicKey string
	Password  string
}

type RegistrationReply struct {
	NodeIp             string
	NodeCIDR           string
	EndpointIPPortPair string
	AllowedIPs         []string
	WGServerPublicKey  string
}

type DeregistrationRequest struct {
	PublicKey string
}

type HeartBeatRequest struct {
	PublicKey string
}

type HeartBeatResponse struct {
	AllowedIPs []string
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
	if r.Password != h.VPNPassword {
		http.Error(w, "Bad password", http.StatusForbidden)
		return
	}
	n, err := h.Registry.Put(r.PublicKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := &RegistrationReply{
		NodeIp:             n.VPNIP,
		NodeCIDR:           n.CIDR,
		EndpointIPPortPair: h.EndpointIPPortPair,
		AllowedIPs:         h.Registry.GetRegisteredIPs(),
		WGServerPublicKey:  h.WGServerPublicKey,
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
}

func (h *HttpApi) unregisterNode(w http.ResponseWriter, req *http.Request) {
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
	if err := h.Registry.Delete(r.PublicKey); err != nil {
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
	n, err := h.Registry.Get(hb.PublicKey)
	if err != nil {
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	n.Beat()

	response := &HeartBeatResponse{
		AllowedIPs: h.Registry.GetRegisteredIPs(),
	}
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
}

func (h *HttpApi) Start(port int, httpCert, httpKey string, running chan struct{}) {
	h.server = &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}

	go func(running chan struct{}) {
		defer func() {
			h.server = nil
		}()
		http.HandleFunc("/register", h.registerNode)
		http.HandleFunc("/unregister", h.unregisterNode)
		http.HandleFunc("/beat", h.heartBeat)

		err := h.server.ListenAndServeTLS(httpCert, httpKey)
		if err != nil {
			fmt.Printf("Fatal error: %s\n", err)
			close(running)
		}
	}(running)
}

func (h *HttpApi) Stop() error {
	if h.running {
		return h.server.Shutdown(context.Background())
	}
	return nil
}
