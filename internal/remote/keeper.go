package remote

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"sync"
	"time"
)

// recheckEvery catches address changes from DHCP, waking from sleep, or switching networks.
const recheckEvery = 30 * time.Second

// Keeper serves a Remote while phone access is on, and only while the computer is on a network it
// was turned on in: it restarts the server when the address changes and stops it off that network.
type Keeper struct {
	r    *Remote
	page http.Handler
	log  *slog.Logger
	// addrs and current (the network the computer is on) are replaced by tests.
	addrs   func(networks []string) ([]netip.Addr, error)
	current func() (string, error)

	mu       sync.Mutex
	srv      *Server
	stop     context.CancelFunc
	onChange func()
}

// NewKeeper returns a Keeper for r, also serving page. Nothing listens until Start or TurnOn.
func NewKeeper(r *Remote, page http.Handler, log *slog.Logger) *Keeper {
	return &Keeper{r: r, page: page, log: log, addrs: interfaceAddrs, current: CurrentNetwork}
}

func interfaceAddrs(networks []string) ([]netip.Addr, error) {
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	return listenAddrs(ifaces, networks), nil
}

// OnChange calls f when the server starts, stops or moves to another address.
func (k *Keeper) OnChange(f func()) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onChange = f
}

// On reports whether sending from a phone is on.
func (k *Keeper) On() bool {
	return len(k.r.networks()) > 0
}

// Networks returns the networks phone access was turned on in.
func (k *Keeper) Networks() []string {
	return k.r.networks()
}

// Start serves r if sending from a phone was left on.
func (k *Keeper) Start() {
	if k.On() {
		k.run()
	}
}

// TurnOn turns sending from a phone on, for the network the computer is on now.
func (k *Keeper) TurnOn() error {
	network, err := k.current()
	if err != nil {
		return err
	}
	k.r.SetNetworks([]string{network})
	k.run()
	if len(k.Addresses()) == 0 {
		return ErrNoNetwork
	}
	return nil
}

// TurnOff stops serving and closes pairing. Paired phones stay paired.
func (k *Keeper) TurnOff() {
	k.r.SetNetworks(nil)
	k.r.StopPairing()
	k.Close()
	k.changed()
}

// Addresses returns the URLs phones can reach Snag at; none while not listening.
func (k *Keeper) Addresses() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.srv == nil {
		return nil
	}
	return k.srv.Addresses()
}

// Close stops serving, leaving the setting as it is.
func (k *Keeper) Close() {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.stop != nil {
		k.stop()
		k.stop = nil
	}
	k.closeServer()
}

func (k *Keeper) run() {
	k.mu.Lock()
	if k.stop != nil {
		k.mu.Unlock()
		k.refresh()
		return
	}
	ctx, stop := context.WithCancel(context.Background())
	k.stop = stop
	k.mu.Unlock()
	k.refresh()
	go func() {
		tick := time.NewTicker(recheckEvery)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				k.refresh()
			}
		}
	}()
}

// refresh restarts the server if the computer's addresses in the networks changed.
func (k *Keeper) refresh() {
	want, err := k.addrs(k.r.networks())
	if err != nil {
		k.log.Warn("couldn't read the computer's addresses", "err", err)
		return
	}
	k.mu.Lock()
	if k.stop == nil || slices.Equal(k.listening(), want) {
		k.mu.Unlock()
		return
	}
	k.closeServer()
	if len(want) > 0 {
		srv, err := start(k.r, k.page, k.log, want)
		if err != nil {
			k.log.Warn("couldn't listen for phones", "err", err)
		} else {
			k.srv = srv
			k.log.Info("listening for phones", "addresses", srv.Addresses())
		}
	}
	k.mu.Unlock()
	k.changed()
}

// listening returns the addresses the server listens on; k.mu is held.
func (k *Keeper) listening() []netip.Addr {
	if k.srv == nil {
		return nil
	}
	out := make([]netip.Addr, len(k.srv.lns))
	for i, ln := range k.srv.lns {
		out[i] = ln.Addr().(*net.TCPAddr).AddrPort().Addr().Unmap()
	}
	return out
}

// closeServer stops the server; k.mu is held.
func (k *Keeper) closeServer() {
	if k.srv == nil {
		return
	}
	if err := k.srv.Close(); err != nil {
		k.log.Warn("the server for phones didn't stop cleanly", "err", err)
	}
	k.srv = nil
}

func (k *Keeper) changed() {
	k.mu.Lock()
	f := k.onChange
	k.mu.Unlock()
	if f != nil {
		go f()
	}
}
