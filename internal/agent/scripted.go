package agent

type Scripted struct {
	id string
}

func NewScripted(id string) *Scripted {
	return &Scripted{id: id}
}

func (s *Scripted) ID() string {
	return s.id
}

func (s *Scripted) Decide(snapshot Snapshot) Action {
	return MOVE_E
}

// Oscillating moves north on even ticks and south on odd ticks.
type Oscillating struct {
	id string
}

func NewOscillating(id string) *Oscillating {
	return &Oscillating{id: id}
}

func (o *Oscillating) ID() string {
	return o.id
}

func (o *Oscillating) Decide(snapshot Snapshot) Action {
	// Prefer a typed accessor if available to avoid reflection.

	if t, ok := snapshot.(interface{ TickValue() int }); ok {
		if t.TickValue()%2 == 0 {
			return MOVE_N
		}
		return MOVE_S
	}

	// Fallback: treat as even tick
	return MOVE_N
}
