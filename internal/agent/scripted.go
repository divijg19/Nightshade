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
	// Always move east
	return Action(2) // MOVE_E
}
