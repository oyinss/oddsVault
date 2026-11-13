package api

import "testing"

func intPtr(v int) *int { return &v }

func TestParseMinute(t *testing.T) {
	tests := []struct {
		detail string
		want   *int
	}{
		{detail: "67'", want: intPtr(67)},
		{detail: "45'+2'", want: intPtr(45)},
		{detail: "HT", want: intPtr(45)},
		{detail: "90+4'", want: intPtr(90)},
		// A descriptive string must never be read as a minute.
		{detail: "Fri, November 13th at 12:00 PM EST", want: nil},
		{detail: "", want: nil},
		{detail: "FT", want: nil},
	}
	for _, tc := range tests {
		got := parseMinute(tc.detail)
		switch {
		case got == nil && tc.want == nil:
		case got == nil || tc.want == nil:
			t.Fatalf("parseMinute(%q) = %v, want %v", tc.detail, ptrString(got), ptrString(tc.want))
		case *got != *tc.want:
			t.Fatalf("parseMinute(%q) = %d, want %d", tc.detail, *got, *tc.want)
		}
	}
}

func ptrString(v *int) string {
	if v == nil {
		return "nil"
	}
	return string(rune('0' + *v%10))
}

func TestApplyCornerMetrics(t *testing.T) {
	home, away := 5, 3
	// 8 corners by the 30th minute is a high rate against a line of 10.
	fast := liveCornerMatch{HomeCorners: &home, AwayCorners: &away, Minute: intPtr(30)}
	applyCornerMetrics(&fast, 10)
	if fast.TotalCorners == nil || *fast.TotalCorners != 8 {
		t.Fatalf("total = %v, want 8", fast.TotalCorners)
	}
	if fast.ProjectedTotal == nil || *fast.ProjectedTotal != 24 {
		t.Fatalf("projection = %v, want 24", fast.ProjectedTotal)
	}
	if fast.Pace != "above" {
		t.Fatalf("pace = %q, want above", fast.Pace)
	}

	lowHome, lowAway := 2, 1
	// 3 corners by the 60th minute is behind a line of 10.
	slow := liveCornerMatch{HomeCorners: &lowHome, AwayCorners: &lowAway, Minute: intPtr(60)}
	applyCornerMetrics(&slow, 10)
	if slow.Needed == nil || *slow.Needed != 7 {
		t.Fatalf("needed = %v, want 7", slow.Needed)
	}
	if slow.Pace != "below" {
		t.Fatalf("pace = %q, want below", slow.Pace)
	}

	// Before the 15th minute there is no projection.
	early := liveCornerMatch{HomeCorners: intPtr(1), AwayCorners: intPtr(1), Minute: intPtr(8)}
	applyCornerMetrics(&early, 10)
	if early.ProjectedTotal != nil {
		t.Fatalf("early projection = %v, want nil", early.ProjectedTotal)
	}
	if early.Pace != "unknown" {
		t.Fatalf("early pace = %q, want unknown", early.Pace)
	}

	// Missing counts stay missing.
	missing := liveCornerMatch{Minute: intPtr(55)}
	applyCornerMetrics(&missing, 10)
	if missing.TotalCorners != nil || missing.ProjectedTotal != nil {
		t.Fatalf("missing data produced a value: %+v", missing)
	}
}
