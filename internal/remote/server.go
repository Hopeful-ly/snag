package remote

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxBody      = 8 << 10
	perMinute    = 120 // requests from one address
	maxAddresses = 256 // addresses tracked for the limit before old ones are forgotten
)

// ErrNoNetwork means the computer has no private address in the networks phone access was turned on in.
var ErrNoNetwork = errors.New("not on the network sending from a phone was turned on in")

// Server serves a Remote over HTTP. It listens only on private addresses (10/8, 172.16/12,
// 192.168/16, IPv6 ULA) in the networks it was turned on in, never on a public one. It checks the
// Host header against DNS rebinding, and accepts only JSON with no CORS headers, so other web
// pages can't send it requests (JSON needs a preflight it never grants).
type Server struct {
	remote *Remote
	page   http.Handler
	log    *slog.Logger
	srv    *http.Server
	lns    []net.Listener
	hosts  map[string]bool // accepted Host headers

	mu     sync.Mutex
	counts map[string]*count
}

type count struct {
	start time.Time
	n     int
}

// CurrentNetwork returns the CIDR of the network internet traffic leaves through, which leaves
// out container, VM and VPN networks.
func CurrentNetwork() (string, error) {
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	out, err := outgoingAddr()
	if err != nil {
		return "", ErrNoNetwork
	}
	if n, ok := networkOf(ifaces, out); ok {
		return n, nil
	}
	return "", ErrNoNetwork
}

// outgoingAddr returns the local address of the default route. A UDP "connection" sends nothing.
func outgoingAddr() (netip.Addr, error) {
	conn, err := net.Dial("udp4", "192.0.2.1:9") // TEST-NET-1: never answers, never needs to
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).AddrPort().Addr().Unmap(), nil
}

// networkOf returns the private network, among the computer's, that addr is on.
func networkOf(ifaces []net.Addr, addr netip.Addr) (string, bool) {
	for _, a := range ifaces {
		if p, ok := privatePrefix(a); ok && p.Addr() == addr {
			return p.Masked().String(), true
		}
	}
	return "", false
}

func privatePrefix(a net.Addr) (netip.Prefix, bool) {
	n, ok := a.(*net.IPNet)
	if !ok {
		return netip.Prefix{}, false
	}
	ip, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}
	ip = ip.Unmap()
	ones, _ := n.Mask.Size()
	if !ip.IsPrivate() {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(ip, ones), true
}

// listenAddrs picks the private addresses inside networks.
func listenAddrs(ifaces []net.Addr, networks []string) []netip.Addr {
	var keep []netip.Prefix
	for _, s := range networks {
		if p, err := netip.ParsePrefix(s); err == nil && p.Addr().IsPrivate() {
			keep = append(keep, p)
		}
	}
	var out []netip.Addr
	for _, a := range ifaces {
		p, ok := privatePrefix(a)
		if !ok {
			continue
		}
		for _, n := range keep {
			if n.Contains(p.Addr()) {
				out = append(out, p.Addr())
				break
			}
		}
	}
	return out
}

func start(r *Remote, page http.Handler, log *slog.Logger, addrs []netip.Addr) (*Server, error) {
	s := &Server{remote: r, page: page, log: log, hosts: map[string]bool{}, counts: map[string]*count{}}
	port := r.port()
	for i, ip := range addrs {
		ln, err := net.Listen("tcp", netip.AddrPortFrom(ip, uint16(port)).String())
		if err != nil && i == 0 && port != 0 {
			// The saved port is taken: use a new one; phones must pair again.
			log.Warn("the port for phones is taken, using another", "port", port, "err", err)
			port = 0
			ln, err = net.Listen("tcp", netip.AddrPortFrom(ip, 0).String())
		}
		if err != nil {
			s.closeListeners()
			return nil, err
		}
		if port == 0 {
			port = ln.Addr().(*net.TCPAddr).Port
		}
		s.lns = append(s.lns, ln)
		s.hosts[ln.Addr().String()] = true
	}
	r.setPort(port)
	s.srv = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	for _, ln := range s.lns {
		go func() {
			if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Warn("the server for phones stopped", "err", err)
			}
		}()
	}
	return s, nil
}

// Addresses returns the URLs phones can reach Snag at.
func (s *Server) Addresses() []string {
	out := make([]string, len(s.lns))
	for i, ln := range s.lns {
		out[i] = "http://" + ln.Addr().String()
	}
	return out
}

// Close stops serving and frees all addresses at once, even those Shutdown would leave to their goroutines.
func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.srv.Shutdown(ctx)
	s.closeListeners()
	return err
}

func (s *Server) closeListeners() {
	for _, ln := range s.lns {
		_ = ln.Close()
	}
}

// ServeHTTP checks the Host header (against DNS rebinding) and the rate limit, then routes.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !s.hosts[r.Host] {
		http.Error(w, "wrong address", http.StatusMisdirectedRequest)
		return
	}
	if !s.allow(r.RemoteAddr) {
		http.Error(w, "too many requests; wait a minute", http.StatusTooManyRequests)
		return
	}
	switch r.URL.Path {
	case "/pair":
		s.servePair(w, r)
	case "/api":
		s.serveAPI(w, r)
	default:
		if s.page == nil {
			http.NotFound(w, r)
			return
		}
		s.page.ServeHTTP(w, r)
	}
}

type pairRequest struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type pairResponse struct {
	Token  string `json:"token"`
	Device Device `json:"device"`
}

func (s *Server) servePair(w http.ResponseWriter, r *http.Request) {
	var req pairRequest
	if !readJSON(w, r, &req) {
		return
	}
	token, d, err := s.remote.Pair(req.Code, req.Name)
	switch {
	case errors.Is(err, ErrWrongCode), errors.Is(err, ErrNotPairing):
		writeJSON(w, http.StatusForbidden, Response{Error: err.Error()})
	case err != nil:
		writeJSON(w, http.StatusInternalServerError, Response{Error: err.Error()})
	default:
		s.log.Info("a phone paired", "device", d.ID, "name", d.Name)
		writeJSON(w, http.StatusOK, pairResponse{Token: token, Device: d})
	}
}

func (s *Server) serveAPI(w http.ResponseWriter, r *http.Request) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		writeJSON(w, http.StatusUnauthorized, Response{Error: ErrUnknownDevice.Error()})
		return
	}
	if _, err := s.remote.Device(token); err != nil {
		writeJSON(w, http.StatusUnauthorized, Response{Error: err.Error()})
		return
	}
	var req Request
	if !readJSON(w, r, &req) {
		return
	}
	writeJSON(w, http.StatusOK, s.remote.Handle(req))
}

// allow counts a request from addr and reports whether it is within the limit.
func (s *Server) allow(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.counts[host]
	if c == nil || now.Sub(c.start) >= time.Minute {
		if c == nil && len(s.counts) >= maxAddresses {
			for k, old := range s.counts {
				if now.Sub(old.start) >= time.Minute {
					delete(s.counts, k)
				}
			}
		}
		c = &count{start: now}
		s.counts[host] = c
	}
	c.n++
	return c.n <= perMinute
}

// readJSON decodes a POSTed JSON body into v, writing the error response itself on failure.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return false
	}
	if ct, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";"); strings.TrimSpace(ct) != "application/json" {
		http.Error(w, "send JSON", http.StatusUnsupportedMediaType)
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, Response{Error: "that isn't a request Snag understands"})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
