package healthmon

import (
	"fmt"
	"log"
	"time"

	pb "logstream/agent/pb"
)

// EventEmitter sends circuit state-change events as LogEntry objects.
type EventEmitter struct {
	serviceID   string
	serviceName string
	agentID     string
	sendFn      func(*pb.LogEntry)
}

// NewEventEmitter creates an emitter that calls sendFn for each event.
func NewEventEmitter(serviceID, serviceName, agentID string, sendFn func(*pb.LogEntry)) *EventEmitter {
	return &EventEmitter{
		serviceID:   serviceID,
		serviceName: serviceName,
		agentID:     agentID,
		sendFn:      sendFn,
	}
}

// EmitOpen emits a circuit-open (ERROR) event.
func (e *EventEmitter) EmitOpen(def ServiceDef, state CircuitState) {
	msg := fmt.Sprintf("Serviço %s ficou indisponível (taxa de erro: %.0f%%)", def.Name, state.ErrorRate*100)
	e.emit(pb.LogLevel_ERROR, def.ID, msg, state)
	log.Printf("[healthmon/event] OPEN %s — %s", def.ID, msg)
}

// EmitRecover emits a circuit-close (INFO) event.
func (e *EventEmitter) EmitRecover(def ServiceDef, state CircuitState) {
	dur := ""
	if len(state.Downtimes) > 0 {
		last := state.Downtimes[len(state.Downtimes)-1]
		if last.Duration != nil {
			d := time.Duration(*last.Duration * float64(time.Second))
			dur = fmt.Sprintf(" após %s", d.Round(time.Second))
		}
	}
	msg := fmt.Sprintf("Serviço %s recuperado%s", def.Name, dur)
	e.emit(pb.LogLevel_INFO, def.ID, msg, state)
	log.Printf("[healthmon/event] RECOVER %s — %s", def.ID, msg)
}

func (e *EventEmitter) emit(level pb.LogLevel, extServiceID, message string, state CircuitState) {
	extra := map[string]string{
		"service_id":   extServiceID,
		"service_name": e.serviceName,
		"status":       state.Status,
		"error_rate":   fmt.Sprintf("%.4f", state.ErrorRate),
		"source":       "healthmon.circuit",
	}
	if state.OpenedAt != nil {
		extra["opened_at"] = state.OpenedAt.Format(time.RFC3339)
	}
	if len(state.Downtimes) > 0 {
		last := state.Downtimes[len(state.Downtimes)-1]
		if last.RecoveredAt != nil {
			extra["recovered_at"] = last.RecoveredAt.Format(time.RFC3339)
		}
		if last.Duration != nil {
			extra["downtime_seconds"] = fmt.Sprintf("%.0f", *last.Duration)
		}
	}

	entry := &pb.LogEntry{
		ServiceId: e.serviceID,
		Level:     level,
		Message:   message,
		Timestamp: time.Now().UnixMilli(),
		Module:    "healthmon",
		AgentId:   e.agentID,
		Extra:     extra,
	}
	if e.sendFn != nil {
		e.sendFn(entry)
	}
}
