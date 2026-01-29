package agent

// IntrospectionSnapshot is a timestamped, read-only record of an
// IntrospectionReport. Stored only for human replay UI; it is never used
// to mutate memory or recompute beliefs.
type IntrospectionSnapshot struct {
	Tick   int
	Report IntrospectionReport
}

// fixed-size ring buffer for snapshots. Package-local helper used only by Human.
type snapshotRing struct {
	buf   [32]IntrospectionSnapshot
	count int // number of valid entries (<= len(buf))
	head  int // index of newest entry in buf when count>0
}

// append a snapshot to the ring buffer (overwrites oldest if full)
func (r *snapshotRing) append(s IntrospectionSnapshot) {
	if r.count == 0 {
		r.head = 0
		r.buf[0] = s
		r.count = 1
		return
	}
	next := (r.head + 1) % len(r.buf)
	r.head = next
	r.buf[r.head] = s
	if r.count < len(r.buf) {
		r.count++
	}
}

// getFromNewest returns the snapshot offset positions back from newest: 0=newest
// returns ok=false if offset >= count
func (r *snapshotRing) getFromNewest(offset int) (IntrospectionSnapshot, bool) {
	var empty IntrospectionSnapshot
	if offset < 0 || offset >= r.count { return empty, false }
	idx := (r.head - offset) % len(r.buf)
	if idx < 0 { idx += len(r.buf) }
	return r.buf[idx], true
}

// count returns number of stored snapshots
func (r *snapshotRing) len() int { return r.count }