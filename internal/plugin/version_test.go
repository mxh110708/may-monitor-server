package plugin

import (
	"github.com/komari-monitor/komari/utils"
	"testing"
)

func TestCheckKomariVersion(t *testing.T) {
	original := utils.CurrentVersion
	utils.CurrentVersion = "0.0.1"
	t.Cleanup(func() { utils.CurrentVersion = original })
	tests := []struct {
		constraint string
		wantErr    bool
	}{
		{"", false},
		{"0.0.1", false},
		{"v0.0.1", false},
		{">=0.0.1", false},
		{">0.0.1", true},
		{"<1.0.0", false},
		{"<=0.0.1", false},
		{">=99.0.0", true},
		{"0.1", true}, // 0.1.0 > 0.0.1
		{"1", true},   // 1.0.0 > 0.0.1
		{"1.2.3.4", true},
		{"abc", true},
		{">", true},
	}
	for _, tt := range tests {
		err := CheckKomariVersion(tt.constraint)
		if (err != nil) != tt.wantErr {
			t.Errorf("CheckKomariVersion(%q) error = %v, wantErr %v", tt.constraint, err, tt.wantErr)
		}
	}
}

func TestPrereleaseCompatibilityNeverSkipsValidation(t *testing.T) {
	original := utils.CurrentVersion
	t.Cleanup(func() { utils.CurrentVersion = original })
	for _, tt := range []struct {
		have, constraint string
		wantError        bool
	}{
		{"0.1.0-rc.1", ">=99.0.0", true},
		{"0.1.0-rc.1", ">=0.1.0", true},
		{"0.1.0-rc.1", ">=0.0.1", false},
		{"0.1.0-rc.1", "=0.1.0-rc.1", false},
		{"0.1.0-rc.1+build.1", "=0.1.0-rc.1", false},
		{"invalid-server-version", ">=0.0.1", true},
	} {
		utils.CurrentVersion = tt.have
		if err := CheckKomariVersion(tt.constraint); (err != nil) != tt.wantError {
			t.Errorf("have=%q constraint=%q error=%v", tt.have, tt.constraint, err)
		}
	}
}
