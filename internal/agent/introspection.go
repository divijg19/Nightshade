package agent

// IntrospectionReport contains simple counts derived from Memory and tick.
// All calculations are read-only and deterministic.
type IntrospectionReport struct {
	TotalBeliefs   int
	Certain        int
	Recent         int
	Fading         int
	Doubtful       int
	HasScars       bool
}

// Introspect analyzes a Memory and currentTick and returns a report of
// belief bucket counts. This function is pure and performs no mutations.
func Introspect(memory Memory, currentTick int) IntrospectionReport {
	r := IntrospectionReport{}
	// Defensive: handle nil memory
	if memory.tiles == nil {
		return r
	}
	for _, mt := range memory.tiles {
		r.TotalBeliefs++
		age := currentTick - mt.LastSeen
		if age == 0 {
			r.Certain++
		} else if age >= 1 && age <= CautionThreshold {
			r.Recent++
		} else if age > CautionThreshold && age <= ParanoiaThreshold {
			r.Fading++
		} else if age > ParanoiaThreshold {
			r.Doubtful++
		}
		if mt.ScarLevel > 0 {
			r.HasScars = true
		}
	}
	return r
}
