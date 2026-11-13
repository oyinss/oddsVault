package settlement

import "testing"

func TestEvaluate(t *testing.T) {
	line := 2.5
	line3 := 3.0

	tests := []struct {
		name string
		c    candidate
		want string
	}{
		{
			name: "1X2 home win hit",
			c:    candidate{marketType: "match_result", selection: "home", homeScore: 2, awayScore: 1},
			want: "hit",
		},
		{
			name: "1X2 home selection loses to away win",
			c:    candidate{marketType: "match_result", selection: "home", homeScore: 0, awayScore: 1},
			want: "miss",
		},
		{
			name: "1X2 draw",
			c:    candidate{marketType: "match_result", selection: "draw", homeScore: 0, awayScore: 0},
			want: "hit",
		},
		{
			name: "double chance home or draw on a draw",
			c:    candidate{marketType: "double_chance", selection: "home_or_draw", homeScore: 1, awayScore: 1},
			want: "hit",
		},
		{
			name: "double chance draw or away on a home win",
			c:    candidate{marketType: "double_chance", selection: "draw_or_away", homeScore: 1, awayScore: 0},
			want: "miss",
		},
		{
			name: "draw no bet pushes on a draw",
			c:    candidate{marketType: "draw_no_bet", selection: "home", homeScore: 1, awayScore: 1},
			want: "push",
		},
		{
			name: "draw no bet away wins",
			c:    candidate{marketType: "draw_no_bet", selection: "away", homeScore: 0, awayScore: 2},
			want: "hit",
		},
		{
			name: "BTTS yes on 2-1",
			c:    candidate{marketType: "both_teams_to_score", selection: "yes", homeScore: 2, awayScore: 1},
			want: "hit",
		},
		{
			name: "BTTS no on 2-0",
			c:    candidate{marketType: "both_teams_to_score", selection: "no", homeScore: 2, awayScore: 0},
			want: "hit",
		},
		{
			name: "over 2.5 on three goals",
			c:    candidate{marketType: "total_goals", selection: "over", line: &line, homeScore: 2, awayScore: 1},
			want: "hit",
		},
		{
			name: "under 2.5 on two goals",
			c:    candidate{marketType: "total_goals", selection: "under", line: &line, homeScore: 1, awayScore: 1},
			want: "hit",
		},
		{
			name: "total exactly on the line pushes",
			c:    candidate{marketType: "total_goals", selection: "over", line: &line3, homeScore: 2, awayScore: 1},
			want: "push",
		},
		{
			name: "unsupported market is not gradable",
			c:    candidate{marketType: "corners", selection: "over", homeScore: 1, awayScore: 0},
			want: "not_gradable",
		},
		{
			name: "unknown selection is not gradable",
			c:    candidate{marketType: "match_result", selection: "home_or_draw", homeScore: 1, awayScore: 0},
			want: "miss",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, actual, _, _ := evaluate(tc.c)
			if got != tc.want {
				t.Fatalf("status = %q, want %q (actual %q)", got, tc.want, actual)
			}
		})
	}
}

func TestEvaluateMoneyline(t *testing.T) {
	tests := []struct {
		name      string
		home      int
		away      int
		selection string
		status    string
	}{
		{name: "home wins", home: 112, away: 104, selection: "home", status: "hit"},
		{name: "away wins", home: 21, away: 24, selection: "home", status: "miss"},
		{name: "away picked", home: 21, away: 24, selection: "away", status: "hit"},
		{name: "tie refunds", home: 20, away: 20, selection: "home", status: "push"},
		{name: "no draw selection", home: 1, away: 2, selection: "draw", status: "not_gradable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _, _, _ := evaluate(candidate{
				marketType: "moneyline",
				selection:  tc.selection,
				homeScore:  tc.home,
				awayScore:  tc.away,
			})
			if status != tc.status {
				t.Fatalf("status = %q, want %q", status, tc.status)
			}
		})
	}
}

func TestEvaluateCorners(t *testing.T) {
	line := 9.5
	exact := 10.0
	ten := 10

	tests := []struct {
		name   string
		c      candidate
		status string
	}{
		{
			name:   "over 9.5 with ten corners",
			c:      candidate{marketType: "total_corners", selection: "over", line: &line, totalCorners: &ten},
			status: "hit",
		},
		{
			name:   "under 9.5 with ten corners",
			c:      candidate{marketType: "total_corners", selection: "under", line: &line, totalCorners: &ten},
			status: "miss",
		},
		{
			name:   "integer line pushes when exact",
			c:      candidate{marketType: "total_corners", selection: "over", line: &exact, totalCorners: &ten},
			status: "push",
		},
		{
			name:   "missing corner data",
			c:      candidate{marketType: "total_corners", selection: "under", line: &line},
			status: "needs_data",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _, _, _ := evaluate(tc.c)
			if status != tc.status {
				t.Fatalf("status = %q, want %q", status, tc.status)
			}
		})
	}
}

func TestEvaluateMissingLine(t *testing.T) {
	status, _, _, reason := evaluate(candidate{marketType: "total_goals", selection: "over", homeScore: 1, awayScore: 1})
	if status != "not_gradable" || reason != "missing_line" {
		t.Fatalf("got %q/%q", status, reason)
	}
}
