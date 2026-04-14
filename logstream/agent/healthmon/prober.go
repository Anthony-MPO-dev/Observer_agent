package healthmon

import (
	"context"
	"log"
	"net/http"
	"time"
)

// prober runs background pings for a single service circuit.
type prober struct {
	def     ServiceDef
	circuit *Circuit

	lastPingAt *time.Time
	lastPingOK bool

	onOpen    func(def ServiceDef, state CircuitState) // called when circuit opens
	onRecover func(def ServiceDef, state CircuitState) // called when circuit closes
}

func newProber(def ServiceDef, c *Circuit, onOpen, onRecover func(ServiceDef, CircuitState)) *prober {
	return &prober{
		def:       def,
		circuit:   c,
		onOpen:    onOpen,
		onRecover: onRecover,
	}
}

// Run starts the ping loop; blocks until ctx is done.
func (p *prober) Run(ctx context.Context) {
	ticker := time.NewTicker(p.def.pingInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.ping(ctx)
		}
	}
}

func (p *prober) ping(ctx context.Context) {
	url := p.def.BaseURL + p.def.HealthPath
	pCtx, cancel := context.WithTimeout(ctx, p.def.pingTimeout)
	defer cancel()

	method := p.def.HealthMethod
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(pCtx, method, url, nil)
	if err != nil {
		p.recordPing(false)
		return
	}

	// Apply configured headers (e.g. Authorization for RabbitMQ)
	for k, v := range p.def.HealthHeaders {
		req.Header.Set(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		p.recordPing(false)
		return
	}
	resp.Body.Close()

	// Determine if status code is acceptable
	ok := false
	if len(p.def.AcceptStatus) > 0 {
		for _, code := range p.def.AcceptStatus {
			if resp.StatusCode == code {
				ok = true
				break
			}
		}
	} else {
		// Default: anything below 500 is considered OK (service is reachable)
		ok = resp.StatusCode < 500
	}
	p.recordPing(ok)
}

func (p *prober) recordPing(ok bool) {
	now := time.Now()
	p.lastPingAt = &now
	p.lastPingOK = ok

	transitioned := p.circuit.PingResult(ok)
	if transitioned {
		log.Printf("[healthmon] %s ping OK → HALF_OPEN", p.def.ID)
	}
}

func (p *prober) LastPingAt() *time.Time { return p.lastPingAt }
func (p *prober) LastPingOK() bool       { return p.lastPingOK }
