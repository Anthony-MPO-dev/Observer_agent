package healthmon

import "sync"

// registry stores all registered circuits and probers indexed by service ID.
type registry struct {
	mu       sync.RWMutex
	defs     map[string]ServiceDef
	circuits map[string]*Circuit
	probers  map[string]*prober
	order    []string // insertion order for /status
}

func newRegistry() *registry {
	return &registry{
		defs:     make(map[string]ServiceDef),
		circuits: make(map[string]*Circuit),
		probers:  make(map[string]*prober),
	}
}

func (r *registry) register(def ServiceDef, c *Circuit, p *prober) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs[def.ID] = def
	r.circuits[def.ID] = c
	r.probers[def.ID] = p
	r.order = append(r.order, def.ID)
}

func (r *registry) circuit(id string) (*Circuit, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.circuits[id]
	return c, ok
}

func (r *registry) def(id string) (ServiceDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.defs[id]
	return d, ok
}

func (r *registry) prober(id string) (*prober, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.probers[id]
	return p, ok
}

func (r *registry) allStates() []CircuitState {
	r.mu.RLock()
	ids := append([]string{}, r.order...)
	r.mu.RUnlock()

	states := make([]CircuitState, 0, len(ids))
	for _, id := range ids {
		c, ok := r.circuit(id)
		if !ok {
			continue
		}
		st := c.Status()
		if p, ok := r.prober(id); ok {
			st.LastPingAt = p.LastPingAt()
			st.LastPingOK = p.LastPingOK()
		}
		states = append(states, st)
	}
	return states
}
