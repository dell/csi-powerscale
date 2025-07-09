package identifiers

import (
	"testing"

	"github.com/google/uuid"
)

func TestGetNewUUID(t *testing.T) {
	id, err := GetNewUUID()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if _, err := uuid.Parse(id); err != nil {
		t.Errorf("expected valid UUID, got %s", id)
	}
}
