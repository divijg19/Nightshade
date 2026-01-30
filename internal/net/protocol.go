package net

import (
	"encoding/binary"
	"encoding/json"
	"io"
)

// Simple length-prefixed JSON frame helper. 4-byte big-endian length then JSON.
func ReadFrame(r io.Reader, v interface{}) error {
	var lenb [4]byte
	if _, err := io.ReadFull(r, lenb[:]); err != nil {
		return err
	}
	l := binary.BigEndian.Uint32(lenb[:])
	buf := make([]byte, l)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	return json.Unmarshal(buf, v)
}

func WriteFrame(w io.Writer, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var lenb [4]byte
	binary.BigEndian.PutUint32(lenb[:], uint32(len(b)))
	if _, err := w.Write(lenb[:]); err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
