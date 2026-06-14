package model

import (
	"testing"
)

func TestSetJobOrigin(t *testing.T) {
	p := &Pipeline{
		Jobs: map[string]Job{
			"with-file": {Name: "with-file", File: "existing.yml"},
			"no-file":   {Name: "no-file"},
		},
	}
	p.SetJobOrigin("default.yml")

	if p.Jobs["with-file"].File != "existing.yml" {
		t.Error("job with existing File should not be overwritten")
	}
	if p.Jobs["no-file"].File != "default.yml" {
		t.Errorf("job without File should be set: got %q", p.Jobs["no-file"].File)
	}
}
