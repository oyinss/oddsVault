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

func TestEvaluateMissingLine(t *testing.T) {
	status, _, _, reason := evaluate(candidate{marketType: "total_goals", selection: "over", homeScore: 1, awayScore: 1})
	if status != "not_gradable" || reason != "missing_line" {
		t.Fatalf("got %q/%q", status, reason)
	}
}
