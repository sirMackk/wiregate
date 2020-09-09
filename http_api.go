package wiregate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
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
		log.Errorf("registerNode received request with method %s, expected POST from %s", req.Method, req.RemoteAddr)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	decoder := json.NewDecoder(req.Body)
	var r RegistrationRequest
	err := decoder.Decode(&r)
	if err != nil {
		log.Errorf("registerNode received incorrect json request from %s: %v", req.RemoteAddr, err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
	if r.Password != h.VPNPassword {
		log.Info("registerNode received request with bad password from %s", req.RemoteAddr)
		http.Error(w, "Bad password", http.StatusForbidden)
		return
	}
	n, err := h.Registry.Put(r.PublicKey)
	if err != nil {
		log.Errorf("registerNode unable to service request from %s (pubkey: %s) due to: %s", req.RemoteAddr, r.PublicKey, err)
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
	log.Debugf("registerNode preparing registration repsonse to %s: %#v", req.RemoteAddr, response)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Errorf("registerNode response to %s failed: %s", req.RemoteAddr, err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
	log.Infof("Successfully registered node %s/%s with pubkey %s as requested by %s", n.VPNIP, n.CIDR, r.PublicKey, req.RemoteAddr)
}

func (h *HttpApi) unregisterNode(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodDelete {
		log.Errorf("unregisterNode received request with method %s, expected POST from %s", req.Method, req.RemoteAddr)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	decoder := json.NewDecoder(req.Body)
	var r DeregistrationRequest
	err := decoder.Decode(&r)
	if err != nil {
		log.Errorf("unregisterNode received incorrect json from %s: %v", req.RemoteAddr, err)
		http.Error(w, "Error while decoding json", http.StatusInternalServerError)
		return
	}
	if err := h.Registry.Delete(r.PublicKey); err != nil {
		log.Errorf("unregisterNode unable to service request from %s (pubkey %s) due to: %s", req.RemoteAddr, r.PublicKey, err)
		http.Error(w, "Node not found", http.StatusNotFound)
	} else {
		log.Infof("Successfully unregistered node with pubkey: %s", r.PublicKey)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HttpApi) heartBeat(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		log.Errorf("heartBeat received request with method %s, expected POST from %s", req.Method, req.RemoteAddr)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	decoder := json.NewDecoder(req.Body)
	var hb HeartBeatRequest
	err := decoder.Decode(&hb)
	if err != nil {
		log.Errorf("heartBeat received incorrect json from %s", req.RemoteAddr)
		http.Error(w, "Error while decoding json", http.StatusInternalServerError)
		return
	}
	n, err := h.Registry.Get(hb.PublicKey)
	if err != nil {
		log.Errorf("heartBeat unable to service request from %s (pubkey %s) due to %s", req.RemoteAddr, hb.PublicKey, err)
		http.Error(w, "Node not found", http.StatusNotFound)
		return
	}
	n.Beat()

	response := &HeartBeatResponse{
		AllowedIPs: h.Registry.GetRegisteredIPs(),
	}

	log.Debugf("heartBeat preparing response to %s: %#v", req.RemoteAddr, response)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Errorf("heartBeat response to %s (pubkey %s) due to: %s", req.RemoteAddr, hb.PublicKey, err)
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}
	log.Infof("Successfully registered beat for pubkey %s request by %s", hb.PublicKey, req.RemoteAddr)
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

		log.Debugf("TLS HTTP server using cert: %s and key: %s", httpCert, httpKey)
		log.Infof("Starting server on address: %v", h.server.Addr)
		err := h.server.ListenAndServeTLS(httpCert, httpKey)
		if err != nil {
			log.Errorf("TLS HTTP server error: %s\n", err)
			close(running)
		}
	}(running)
}

func (h *HttpApi) Stop() error {
	log.Info("Stopping TLS HTTP server")
	if h.running {
		return h.server.Shutdown(context.Background())
	}
	return nil
}
