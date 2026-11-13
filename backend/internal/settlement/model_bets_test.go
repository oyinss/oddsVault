package settlement

import "testing"

func TestModelBetStatus(t *testing.T) {
	tests := []struct {
		name  string
		tally legTally
		want  string
	}{
		{name: "no legs", tally: legTally{}, want: "unknown"},
		{name: "waiting on data", tally: legTally{legs: 2, hits: 1, unresolved: 1}, want: "pending"},
		{name: "a miss loses the bet", tally: legTally{legs: 2, hits: 1, misses: 1}, want: "lost"},
		{name: "every leg hit", tally: legTally{legs: 2, hits: 2}, want: "won"},
		{name: "void leg drops out", tally: legTally{legs: 2, hits: 1, voids: 1}, want: "won"},
		{name: "every leg void", tally: legTally{legs: 2, voids: 2}, want: "void"},
		{name: "miss beats void", tally: legTally{legs: 2, misses: 1, voids: 1}, want: "lost"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := modelBetStatus(tc.tally); got != tc.want {
				t.Fatalf("modelBetStatus(%+v) = %q, want %q", tc.tally, got, tc.want)
			}
		})
	}
}
