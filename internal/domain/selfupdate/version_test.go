package selfupdate

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in      string
		want    Version
		wantErr bool
	}{
		{"v1.2.3", Version{1, 2, 3}, false},
		{"1.2.3", Version{1, 2, 3}, false},
		{"v0.0.1", Version{0, 0, 1}, false},
		{"v1.2.3-rc1", Version{1, 2, 3}, false},
		{"v1.2.3+build5", Version{1, 2, 3}, false},
		{"dev", Version{}, true},
		{"v1.2", Version{}, true},
		{"", Version{}, true},
	}

	for _, c := range cases {
		got, err := ParseVersion(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseVersion(%q) = %+v, want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseVersion(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	v := Version{Major: 1, Minor: 2, Patch: 3}
	if got := v.String(); got != "v1.2.3" {
		t.Fatalf("String() = %q, want %q", got, "v1.2.3")
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current string
		latest  string
		want    bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.1", "v1.0.0", false},
		{"v1.0.0", "v1.0.0", false},
		{"v1.2.0", "v1.10.0", true},
		{"v1.9.9", "v2.0.0", true},
		{"v2.0.0", "v1.9.9", false},
	}

	for _, c := range cases {
		got, err := IsNewer(c.current, c.latest)
		if err != nil {
			t.Fatalf("IsNewer(%q, %q): unexpected error: %v", c.current, c.latest, err)
		}
		if got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestIsNewer_InvalidVersion(t *testing.T) {
	if _, err := IsNewer("dev", "v1.0.0"); err == nil {
		t.Fatalf("expected an error for an unparsable current version")
	}
}
