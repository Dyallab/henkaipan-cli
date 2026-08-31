package severity

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Level
	}{
		{"", None},
		{"none", None},
		{"critical", Critical},
		{"HIGH", High},
		{" Medium ", Medium},
		{"low", Low},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) unexpected error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("Parse(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseUnknown(t *testing.T) {
	if _, err := Parse("bogus"); err == nil {
		t.Fatal("expected error for unknown severity")
	}
}

func TestWeight(t *testing.T) {
	if Weight(Critical) != 4 || Weight(High) != 3 ||
		Weight(Medium) != 2 || Weight(Low) != 1 || Weight(None) != 0 {
		t.Fatal("weight table does not match action contract")
	}
}

func TestCountsExceedsThreshold(t *testing.T) {
	cases := []struct {
		name  string
		c     Counts
		level Level
		want  bool
	}{
		{"empty", Counts{}, Low, false},
		{"none", Counts{}, None, false},
		{"critical vs high", Counts{Critical: 1}, High, true},
		{"medium vs high", Counts{Medium: 1}, High, false},
		{"low vs low", Counts{Low: 1}, Low, true},
		{"high vs critical", Counts{High: 1}, Critical, false},
	}
	for _, c := range cases {
		if got := c.c.ExceedsThreshold(c.level); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFromStrings(t *testing.T) {
	c := FromStrings([]string{"critical", "HIGH", "high", "low", "garbage", "medium"})
	want := Counts{Critical: 1, High: 2, Medium: 1, Low: 1}
	if c != want {
		t.Errorf("got %+v, want %+v", c, want)
	}
	if c.Total() != 5 {
		t.Errorf("total = %d, want 5", c.Total())
	}
}