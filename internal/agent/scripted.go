package agent

type Scripted struct {
	id     string
	memory *Memory
}

func NewScripted(id string) *Scripted {
	return &Scripted{id: id,
		memory: NewMemory(),
	}
}

func (s *Scripted) ID() string {
	return s.id
}

func (s *Scripted) Decide(snapshot Snapshot) Action {
	if s.memory != nil {
		s.memory.UpdateFromSnapshot(snapshot)
	}
	return MOVE_E
}

// Memory exposes the agent's memory for external inspection in tools/tests.
func (s *Scripted) Memory() *Memory {
	return s.memory
}

// Oscillating moves north on even ticks and south on odd ticks.
type Oscillating struct {
	id     string
	memory *Memory
}

func NewOscillating(id string) *Oscillating {
	return &Oscillating{id: id,
		memory: NewMemory(),
	}
}

func (o *Oscillating) ID() string {
	return o.id
}

func (o *Oscillating) Decide(snapshot Snapshot) Action {
	// Prefer a typed accessor if available to avoid reflection.
	{
		if o.memory != nil {
			o.memory.UpdateFromSnapshot(snapshot)
		}

		if t, ok := snapshot.(interface{ TickValue() int }); ok {
			if t.TickValue()%2 == 0 {
				return MOVE_N
			}
			return MOVE_S
		}

		// Fallback: treat as even tick
		return MOVE_N
	}
}

// Memory exposes the agent's memory for external inspection in tools/tests.
func (o *Oscillating) Memory() *Memory {
	return o.memory
}
