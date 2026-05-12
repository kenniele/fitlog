package domain

import "time"

// Note is a free-form log entry the user writes via /log. Kind is one of a
// small set ("weight", "note", "symptom"); value is set only for numeric
// kinds (e.g. weight); body holds the free text.
type Note struct {
	ID    int64
	Ts    time.Time
	Kind  string
	Value *float64
	Body  string
}

// Common note kinds. The handler accepts free strings too, falling back to
// NoteKindNote, but these are the canonical ones rendered specially.
const (
	NoteKindWeight  = "weight"
	NoteKindWaist   = "waist"
	NoteKindNote    = "note"
	NoteKindSymptom = "symptom"
)
