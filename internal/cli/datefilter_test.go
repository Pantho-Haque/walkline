package cli

import (
	"testing"
)

func TestBuildDateFilter_MutualExclusivity(t *testing.T) {
	tests := []struct {
		name    string
		since   string
		until   string
		at      string
		wantErr bool
	}{
		{"none set", "", "", "", false},
		{"since only", "2024-01-01", "", "", false},
		{"until only", "", "2024-01-01", "", false},
		{"at only", "", "", "2024-01-01", false},
		{"since + until", "2024-01-01", "2024-12-31", "", true},
		{"since + at", "2024-01-01", "", "2024-06-15", true},
		{"until + at", "", "2024-12-31", "2024-06-15", true},
		{"all three", "2024-01-01", "2024-12-31", "2024-06-15", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildDateFilter(tt.since, tt.until, tt.at)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildDateFilter(%q, %q, %q) error = %v, wantErr %v", tt.since, tt.until, tt.at, err, tt.wantErr)
			}
		})
	}
}

func TestBuildDateFilter_SinceBareDate(t *testing.T) {
	f, err := BuildDateFilter("2024-01-15", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Since != "2024-01-15T00:00:00Z" {
		t.Errorf("expected Since=2024-01-15T00:00:00Z, got %q", f.Since)
	}
}

func TestBuildDateFilter_SinceRFC3339(t *testing.T) {
	f, err := BuildDateFilter("2024-01-15T12:30:00Z", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Since != "2024-01-15T12:30:00Z" {
		t.Errorf("expected Since=2024-01-15T12:30:00Z, got %q", f.Since)
	}
}

func TestBuildDateFilter_UntilBareDate(t *testing.T) {
	f, err := BuildDateFilter("", "2024-01-15", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Until != "2024-01-15T23:59:59Z" {
		t.Errorf("expected Until=2024-01-15T23:59:59Z, got %q", f.Until)
	}
}

func TestBuildDateFilter_UntilRFC3339(t *testing.T) {
	f, err := BuildDateFilter("", "2024-01-15T12:30:00Z", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.Until != "2024-01-15T12:30:00Z" {
		t.Errorf("expected Until=2024-01-15T12:30:00Z, got %q", f.Until)
	}
}

func TestBuildDateFilter_AtBareDate(t *testing.T) {
	f, err := BuildDateFilter("", "", "2024-01-15")
	if err != nil {
		t.Fatal(err)
	}
	if f.Since != "2024-01-15T00:00:00Z" {
		t.Errorf("expected Since=2024-01-15T00:00:00Z, got %q", f.Since)
	}
	if f.Until != "2024-01-15T23:59:59Z" {
		t.Errorf("expected Until=2024-01-15T23:59:59Z, got %q", f.Until)
	}
}

func TestBuildDateFilter_AtRFC3339(t *testing.T) {
	f, err := BuildDateFilter("", "", "2024-01-15T14:30:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if f.Since != "2024-01-15T00:00:00Z" {
		t.Errorf("expected Since=2024-01-15T00:00:00Z, got %q", f.Since)
	}
	if f.Until != "2024-01-15T23:59:59Z" {
		t.Errorf("expected Until=2024-01-15T23:59:59Z, got %q", f.Until)
	}
}

func TestBuildDateFilter_InvalidDate(t *testing.T) {
	_, err := BuildDateFilter("not-a-date!", "", "")
	if err == nil {
		t.Error("expected error for invalid date")
	}
}

func TestIsBareDate(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"2024-01-15", true},
		{"2024-01-15T00:00:00Z", false},
		{"2024-1-15", false},
		{"20240115", false},
		{"abc", false},
	}
	for _, tt := range tests {
		if got := isBareDate(tt.input); got != tt.want {
			t.Errorf("isBareDate(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
