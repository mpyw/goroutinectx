package deriver

import (
	"strings"
	"testing"
)

// TestNewMatcher checks the OR groups NewMatcher builds, and the flag value
// Original keeps for messages.
func TestNewMatcher(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher([]string{
		"example.com/newrelic.Transaction.NewGoroutine + example.com/newrelic.NewContext",
		"example.com/apm.NewGoroutineContext",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got [][]string
	for _, group := range m.OrGroups {
		var names []string
		for _, spec := range group {
			names = append(names, strings.Join([]string{spec.PkgPath, spec.TypeName, spec.FuncName}, " "))
		}
		got = append(got, names)
	}
	want := [][]string{
		{"example.com/newrelic Transaction NewGoroutine", "example.com/newrelic  NewContext"},
		{"example.com/apm  NewGoroutineContext"},
	}
	if len(got) != len(want) {
		t.Fatalf("got groups %v, want %v", got, want)
	}
	for i := range want {
		if strings.Join(got[i], ",") != strings.Join(want[i], ",") {
			t.Errorf("group %d is %v, want %v", i+1, got[i], want[i])
		}
	}
	if wantOriginal := "example.com/newrelic.Transaction.NewGoroutine + example.com/newrelic.NewContext,example.com/apm.NewGoroutineContext"; m.Original != wantOriginal {
		t.Errorf("Original is %q, want %q", m.Original, wantOriginal)
	}
}

// TestNewMatcherEmpty checks that no groups give an empty matcher.
func TestNewMatcherEmpty(t *testing.T) {
	t.Parallel()

	m, err := NewMatcher(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !m.IsEmpty() {
		t.Errorf("got groups %v, want none", m.OrGroups)
	}
}

// TestNewMatcherRejects checks the errors NewMatcher gives.
func TestNewMatcherRejects(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups []string
		want   string
	}{
		{"an empty group", []string{"example.com/apm.F", ""}, "-goroutine-deriver: group 2 is empty"},
		{"a blank group", []string{"  "}, "-goroutine-deriver: group 1 is empty"},
		{"an empty function after +", []string{"example.com/apm.F+"}, `-goroutine-deriver: "example.com/apm.F+": a function joined by + is empty`},
		{"an empty function between +", []string{"example.com/apm.F++example.com/apm.G"}, "a function joined by + is empty"},
		{"a group with no import path", []string{"example.com/apm.F+apm.G"}, `-goroutine-deriver: "apm.G": "apm" is not an import path`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := NewMatcher(tt.groups); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("got error %v, want one containing %q", err, tt.want)
			}
		})
	}
}
