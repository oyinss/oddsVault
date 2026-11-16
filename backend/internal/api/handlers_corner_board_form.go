package api

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// boardTeamForm is the last-five corner record for one side of a board row. It
// is the same shape the match detail page uses, minus the team identity, so the
// board can print the numbers without a second request per card.
type boardTeamForm struct {
	Matches    []formMatch        `json:"matches"`
	Sample     int                `json:"sample"`
	AvgFor     float64            `json:"avg_for"`
	AvgAgainst float64            `json:"avg_against"`
	AvgTotal   float64            `json:"avg_total"`
	OverRates  map[string]float64 `json:"over_rates"`
}

func newBoardTeamForm() *boardTeamForm {
	return &boardTeamForm{
		Matches:   []formMatch{},
		OverRates: map[string]float64{},
	}
}

// attachBoardTeamForms fills the last five finished matches with corner stats
// for both sides of every live row. One query covers every team on the board,
// so a forty-match page does not make eighty round trips.
func (s *Server) attachBoardTeamForms(ctx context.Context, matches []liveCornerMatch) {
	if len(matches) == 0 {
		return
	}
	unique := make(map[int64]struct{}, len(matches)*2)
	for _, match := range matches {
		// Short-format games reuse real club names, so a team id match there
		// would print eleven-a-side form next to a five-a-side score.
		if strings.Contains(match.CompetitionSlug, "short-football") {
			continue
		}
		if match.Home.ID != 0 {
			unique[match.Home.ID] = struct{}{}
		}
		if match.Away.ID != 0 {
			unique[match.Away.ID] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return
	}
	teamIDs := make([]int64, 0, len(unique))
	for id := range unique {
		teamIDs = append(teamIDs, id)
	}

	rows, err := s.pool.Query(ctx, `
		WITH ranked AS (
			SELECT ep.team_id, e.id AS event_id, e.start_date,
			       opp.name AS opponent, (ep.role = 'home') AS home,
			       s.corners AS corners_for, os.corners AS corners_against,
			       row_number() OVER (
			           PARTITION BY ep.team_id
			           ORDER BY e.start_date DESC, e.id DESC
			       ) AS rn
			FROM event_participants ep
			JOIN events e ON e.id = ep.event_id AND e.status = 'finished'
			JOIN team_match_stats s ON s.event_id = e.id AND s.team_id = ep.team_id
			JOIN event_participants ope ON ope.event_id = e.id AND ope.role <> ep.role
			JOIN team_match_stats os ON os.event_id = e.id AND os.team_id = ope.team_id
			JOIN teams opp ON opp.id = ope.team_id
			WHERE ep.team_id = ANY($1::bigint[])
			  AND s.corners IS NOT NULL
			  AND os.corners IS NOT NULL
		)
		SELECT team_id, event_id, start_date, opponent, home, corners_for, corners_against
		FROM ranked
		WHERE rn <= 5
		ORDER BY team_id, start_date DESC, event_id DESC`, teamIDs)
	if err != nil {
		s.log.ErrorContext(ctx, "board team form", "error", err.Error())
		return
	}
	defer rows.Close()

	forms := make(map[int64]*boardTeamForm, len(teamIDs))
	for rows.Next() {
		var teamID int64
		var item formMatch
		var day time.Time
		if err := rows.Scan(&teamID, &item.EventID, &day, &item.Opponent, &item.Home,
			&item.CornersFor, &item.CornersAgainst); err != nil {
			s.log.ErrorContext(ctx, "scan board team form", "error", err.Error())
			return
		}
		item.Date = day.Format("2006-01-02")
		item.Total = item.CornersFor + item.CornersAgainst
		form := forms[teamID]
		if form == nil {
			form = newBoardTeamForm()
			forms[teamID] = form
		}
		form.Matches = append(form.Matches, item)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(ctx, "iterate board team form", "error", err.Error())
		return
	}

	lines := []float64{8.5, 9.5, 10.5, 11.5}
	for _, form := range forms {
		if len(form.Matches) == 0 {
			continue
		}
		var sumFor, sumAgainst, sumTotal float64
		over := map[float64]int{}
		for _, item := range form.Matches {
			sumFor += float64(item.CornersFor)
			sumAgainst += float64(item.CornersAgainst)
			sumTotal += float64(item.Total)
			for _, line := range lines {
				if float64(item.Total) > line {
					over[line]++
				}
			}
		}
		n := float64(len(form.Matches))
		form.Sample = len(form.Matches)
		form.AvgFor = round2(sumFor / n)
		form.AvgAgainst = round2(sumAgainst / n)
		form.AvgTotal = round2(sumTotal / n)
		for _, line := range lines {
			form.OverRates[strconv.FormatFloat(line, 'f', 1, 64)] =
				round2(float64(over[line]) / n)
		}
	}

	// Half splits for the matches in these lists, where we captured them.
	eventIDs := make([]int64, 0, len(forms)*5)
	seen := make(map[int64]bool, len(forms)*5)
	for _, form := range forms {
		for _, item := range form.Matches {
			if !seen[item.EventID] {
				seen[item.EventID] = true
				eventIDs = append(eventIDs, item.EventID)
			}
		}
	}
	firstHalf := s.firstHalfTotals(ctx, eventIDs)
	for _, form := range forms {
		for index := range form.Matches {
			item := &form.Matches[index]
			first, ok := firstHalf[item.EventID]
			if !ok {
				continue
			}
			second := item.Total - first
			if second < 0 {
				continue
			}
			item.FirstHalf = &first
			item.SecondHalf = &second
		}
	}

	for index := range matches {
		if form, ok := forms[matches[index].Home.ID]; ok {
			matches[index].HomeForm = form
		}
		if form, ok := forms[matches[index].Away.ID]; ok {
			matches[index].AwayForm = form
		}
	}
}
