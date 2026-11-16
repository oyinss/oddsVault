package api

import (
	"context"
	"fmt"
	"sort"
)

// matchShape is the information behind the corner count: when the goals came,
// who was leading, and how much each side shot. It explains a result instead of
// describing it.
type shapeGoal struct {
	Minute  int    `json:"minute"`
	Home    bool   `json:"home"`
	Player  string `json:"player"`
	OwnGoal bool   `json:"own_goal"`
}

type shapeTeam struct {
	Shots         *int     `json:"shots"`
	ShotsOnTarget *int     `json:"shots_on_target"`
	BigChances    *int     `json:"big_chances"`
	Passes        *int     `json:"passes"`
	XG            *float64 `json:"xg"`
	Possession    *float64 `json:"possession"`
}

type matchShape struct {
	Goals         []shapeGoal `json:"goals"`
	Home          shapeTeam   `json:"home"`
	Away          shapeTeam   `json:"away"`
	Favourite     *string     `json:"favourite"`
	FavouriteOdds *float64    `json:"favourite_odds"`
	StateAt60     *string     `json:"state_at_60"`
	HomeScore     *int        `json:"home_score"`
	AwayScore     *int        `json:"away_score"`
}

// matchShape loads the goal timeline, both teams' shooting and the favourite's
// state at 60 minutes. It returns nil when nothing was stored.
func (s *Server) matchShape(ctx context.Context, eventID int64) (*matchShape, error) {
	shape := &matchShape{Goals: []shapeGoal{}}

	rows, err := s.pool.Query(ctx, `
		SELECT minute, is_home, COALESCE(player, ''), own_goal
		FROM match_goal_events WHERE event_id = $1 ORDER BY minute`, eventID)
	if err != nil {
		return nil, fmt.Errorf("shape goals: %w", err)
	}
	for rows.Next() {
		var goal shapeGoal
		if err := rows.Scan(&goal.Minute, &goal.Home, &goal.Player, &goal.OwnGoal); err != nil {
			rows.Close()
			return nil, err
		}
		shape.Goals = append(shape.Goals, goal)
	}
	rows.Close()

	statRows, err := s.pool.Query(ctx, `
		SELECT t.team_id, t.shots, t.shots_on_target, t.big_chances,
		       t.passes, t.xg, t.possession
		FROM team_attack_stats t WHERE t.event_id = $1`, eventID)
	if err != nil {
		return nil, fmt.Errorf("shape stats: %w", err)
	}
	defer statRows.Close()

	statsByTeam := map[int64]shapeTeam{}
	for statRows.Next() {
		var teamID int64
		var line shapeTeam
		if err := statRows.Scan(&teamID, &line.Shots, &line.ShotsOnTarget,
			&line.BigChances, &line.Passes, &line.XG, &line.Possession); err != nil {
			return nil, err
		}
		statsByTeam[teamID] = line
	}

	var homeTeamID, awayTeamID int64
	if err := s.pool.QueryRow(ctx, `
		SELECT max(team_id) FILTER (WHERE role = 'home'),
		       max(team_id) FILTER (WHERE role = 'away')
		FROM event_participants WHERE event_id = $1`, eventID).Scan(&homeTeamID, &awayTeamID); err != nil {
		return nil, err
	}
	shape.Home = statsByTeam[homeTeamID]
	shape.Away = statsByTeam[awayTeamID]

	if err := s.pool.QueryRow(ctx, `
		SELECT home_score, away_score FROM event_results WHERE event_id = $1`, eventID).
		Scan(&shape.HomeScore, &shape.AwayScore); err != nil {
		// A result may not exist yet; the timeline still stands on its own.
		shape.HomeScore, shape.AwayScore = nil, nil
	}

	// Closing 1X2 favourite: lowest price across the three selections.
	favRows, err := s.pool.Query(ctx, `
		SELECT ms.selection, os.decimal_odds
		FROM markets m
		JOIN market_types mt ON mt.id = m.market_type_id AND mt.slug = 'match_result'
		JOIN market_selections ms ON ms.market_id = m.id
		JOIN odds_snapshots os ON os.event_id = m.event_id
		    AND os.selection_id = ms.id AND os.availability_status = 'available'
		WHERE m.event_id = $1
		ORDER BY os.captured_at DESC`, eventID)
	if err != nil {
		return nil, fmt.Errorf("shape odds: %w", err)
	}
	prices := map[string]float64{}
	for favRows.Next() {
		var side string
		var odds float64
		if err := favRows.Scan(&side, &odds); err != nil {
			favRows.Close()
			return nil, err
		}
		if _, seen := prices[side]; !seen {
			prices[side] = odds
		}
	}
	favRows.Close()

	if len(prices) > 0 {
		sides := make([]string, 0, len(prices))
		for side := range prices {
			sides = append(sides, side)
		}
		sort.Slice(sides, func(i, j int) bool { return prices[sides[i]] < prices[sides[j]] })
		favourite := sides[0]
		shape.Favourite = &favourite
		odds := prices[favourite]
		shape.FavouriteOdds = &odds

		if state := favouriteState(shape.Goals, favourite, 60); state != "" {
			shape.StateAt60 = &state
		}
	}

	if len(shape.Goals) == 0 && shape.Home.Shots == nil && shape.Away.Shots == nil {
		return nil, nil
	}
	return shape, nil
}

// favouriteState replays the goal timeline to say whether the favourite was
// leading, level or behind at a given minute.
func favouriteState(goals []shapeGoal, favourite string, minute int) string {
	home, away := 0, 0
	for _, goal := range goals {
		if goal.Minute > minute {
			break
		}
		if goal.Home {
			home++
		} else {
			away++
		}
	}
	favHome, favAway := home, away
	if favourite == "away" {
		favHome, favAway = away, home
	}
	switch {
	case favourite == "draw":
		if home == away {
			return "level"
		}
		return "behind"
	case favHome > favAway:
		return "leading"
	case favHome == favAway:
		return "level"
	default:
		return "behind"
	}
}
