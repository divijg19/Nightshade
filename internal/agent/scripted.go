package agent

import "reflect"

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
	// snapshot is an opaque interface. Use reflection to read the exported
	// Tick field (runtime.Snapshot has exported Tick int). This avoids
	// importing the runtime package and avoids circular deps.
	importReflect := func(s Snapshot) int {
		// lazily use reflect to extract Tick
		// keep zero default if anything unexpected
		tick := 0
		if s == nil {
			return tick
		}
		v := reflect.ValueOf(s)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			f := v.FieldByName("Tick")
			if f.IsValid() && f.CanInt() {
				tick = int(f.Int())
			}
		}
		return tick
	}

	tick := importReflect(snapshot)
	if tick%2 == 0 {
		return MOVE_N
	}
	return MOVE_S
}
