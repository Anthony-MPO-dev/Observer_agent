package healthmon

import (
	"context"
	"log"

	pb "logstream/agent/pb"
)

// DependencyStatuses returns the current circuit breaker state for all monitored
// services as pb.DependencyStatus, ready to attach to a HeartbeatRequest.
func (m *Monitor) DependencyStatuses() []*pb.DependencyStatus {
	if m == nil || m.reg == nil {
		return nil
	}
	states := m.reg.allStates()
	result := make([]*pb.DependencyStatus, 0, len(states))
	for _, st := range states {
		def, _ := m.reg.def(st.ServiceID)
		ds := &pb.DependencyStatus{
			ServiceID:     st.ServiceID,
			Name:          def.Name,
			Status:        st.Status,
			ErrorRate:     st.ErrorRate,
			TotalRequests: st.TotalRequests,
			TotalErrors:   st.TotalErrors,
			Essential:     def.Essential,
			Fallbacks:     def.Fallbacks,
			LastPingOK:    st.LastPingOK,
		}
		if st.OpenedAt != nil {
			ms := st.OpenedAt.UnixMilli()
			ds.OpenedAt = &ms
		}
		if st.LastPingAt != nil {
			ms := st.LastPingAt.UnixMilli()
			ds.LastPingAt = &ms
		}
		result = append(result, ds)
	}
	return result
}

// Monitor is the top-level healthmon orchestrator.
// Create one with New(), then call Start(ctx) in a goroutine.
type Monitor struct {
	reg     *registry
	emitter *EventEmitter
	port    string
	defs    []ServiceDef
}

// New creates a Monitor. sendFn is called for each circuit-event LogEntry
// (wire it to sender.Send so events appear in the log dashboard).
func New(defs []ServiceDef, serviceID, serviceName, agentID, port string, sendFn func(*pb.LogEntry)) *Monitor {
	emitter := NewEventEmitter(serviceID, serviceName, agentID, sendFn)
	return &Monitor{
		reg:     newRegistry(),
		emitter: emitter,
		port:    port,
		defs:    defs,
	}
}

// Start initialises all circuits and probers and runs the HTTP server.
// It blocks until ctx is cancelled.
func (m *Monitor) Start(ctx context.Context) {
	if len(m.defs) == 0 {
		log.Printf("[healthmon] no services configured — HEALTHMON_SERVICES not set")
		<-ctx.Done()
		return
	}

	for _, def := range m.defs {
		d := def // capture
		c := newCircuit(d)

		onOpen := func(def ServiceDef, state CircuitState) {
			m.emitter.EmitOpen(def, state)
		}
		onRecover := func(def ServiceDef, state CircuitState) {
			m.emitter.EmitRecover(def, state)
		}

		p := newProber(d, c, onOpen, onRecover)
		m.reg.register(d, c, p)

		go p.Run(ctx)
		log.Printf("[healthmon] registered service %s (%s)", d.ID, d.BaseURL)
	}

	go serveHealthmon(m.port, m)

	<-ctx.Done()
}

// Check returns availability of service `id`, considering fallbacks.
func (m *Monitor) Check(id string) CheckResponse {
	def, ok := m.reg.def(id)
	if !ok {
		return CheckResponse{
			ServiceID: id,
			Available: true, // unknown service → fail open
			Status:    "CLOSED",
			Message:   "serviço não monitorado — permitindo requisição",
		}
	}

	c, _ := m.reg.circuit(id)
	available, _ := c.Check()
	state := c.Status()

	if available {
		return CheckResponse{
			ServiceID: id,
			Available: true,
			Status:    state.Status,
			Essential: def.Essential,
			Message:   "ok",
		}
	}

	// Circuit is OPEN — try fallbacks (only for non-essential services)
	if !def.Essential {
		for _, fbID := range def.Fallbacks {
			fbCircuit, ok := m.reg.circuit(fbID)
			if !ok {
				continue
			}
			fbAvailable, _ := fbCircuit.Check()
			if fbAvailable {
				fb := fbID
				return CheckResponse{
					ServiceID: id,
					Available: true,
					Status:    state.Status,
					Fallback:  &fb,
					Essential: false,
					Message:   def.ID + " indisponível, use " + fbID + " como fallback",
					OpenedAt:  state.OpenedAt,
				}
			}
		}
		// All fallbacks also down
		return CheckResponse{
			ServiceID: id,
			Available: false,
			Status:    state.Status,
			Essential: false,
			Message:   "todos os serviços de dados básicos estão indisponíveis",
			OpenedAt:  state.OpenedAt,
		}
	}

	// Essential service is OPEN
	return CheckResponse{
		ServiceID: id,
		Available: false,
		Status:    state.Status,
		Essential: true,
		Message:   "serviço essencial indisponível — operação bloqueada",
		OpenedAt:  state.OpenedAt,
	}
}

// Report records a request outcome for service `id`.
func (m *Monitor) Report(id string, success bool) {
	c, ok := m.reg.circuit(id)
	if !ok {
		return
	}
	def, _ := m.reg.def(id)
	justOpened := c.Report(success)
	if justOpened {
		state := c.Status()
		m.emitter.EmitOpen(def, state)
	}
}
