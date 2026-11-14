package api

import (
	"context"
	"fmt"
)

// Corner bet history answers one question on the match page: when I have bet
// corner totals on this competition, these teams, or both sides of this fixture
// before, how did it go? It counts legs from the personal bet journal only.
// Accumulator legs are counted as legs but never turned into money, because a
// slip's profit does not belong to one leg.

type cornerBetSide struct {
	Legs    int `json:"legs"`
	Won     int `json:"won"`
	Lost    int `json:"lost"`
	Void    int `json:"void"`
	Pending int `json:"pending"`
	Unknown int `json:"unknown"`
}

type cornerBetMoney struct {
	Bets       int     `json:"bets"`
	Stake      float64 `json:"stake"`
	ProfitLoss float64 `json:"profit_loss"`
}

type cornerBetRecord struct {
	Over      cornerBetSide  `json:"over"`
	Under     cornerBetSide  `json:"under"`
	SingleBet cornerBetMoney `json:"single_bet"`
}

type cornerBetHistory struct {
	Competition *cornerBetRecord `json:"competition"`
	Home        *cornerBetRecord `json:"home"`
	Away        *cornerBetRecord `json:"away"`
	Journal     cornerBetJournal `json:"journal"`
	Note        string           `json:"note"`
}

// cornerBetJournal reports how much of the journal could be attached to a
// stored fixture. A leg that never resolved cannot appear under a team, and
// saying so is better than implying the bet does not exist.
type cornerBetJournal struct {
	CornerLegs int `json:"corner_legs"`
	LinkedLegs int `json:"linked_legs"`
}

func (h *cornerBetHistory) empty() bool {
	for _, record := range []*cornerBetRecord{h.Competition, h.Home, h.Away} {
		if record == nil {
			continue
		}
		if record.Over.Legs > 0 || record.Under.Legs > 0 {
			return false
		}
	}
	return true
}

// cornerBetsByTeam sums corner legs and single-leg money for one team.
func (s *Server) cornerBetsByTeam(ctx context.Context, teamID int64) (*cornerBetRecord, error) {
	return s.cornerBets(ctx,
		`l.event_id IN (SELECT event_id FROM event_participants WHERE team_id = $1)`, teamID)
}

// cornerBetsByCompetition sums corner legs and single-leg money for one league.
func (s *Server) cornerBetsByCompetition(ctx context.Context, competitionID int64) (*cornerBetRecord, error) {
	return s.cornerBets(ctx,
		`l.event_id IN (SELECT id FROM events WHERE competition_id = $1)`, competitionID)
}

// cornerJournalCounts reads the journal-wide corner leg totals.
func (s *Server) cornerJournalCounts(ctx context.Context) (cornerBetJournal, error) {
	var out cornerBetJournal
	err := s.pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE event_id IS NOT NULL)
		FROM personal_bet_legs
		WHERE market_family = 'corners_total'`).Scan(&out.CornerLegs, &out.LinkedLegs)
	return out, err
}

func (s *Server) cornerBets(ctx context.Context, scope string, id int64) (*cornerBetRecord, error) {
	record := &cornerBetRecord{}

	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.market_side, l.leg_status, count(*)
		FROM personal_bet_legs l
		WHERE l.market_family = 'corners_total'
		  AND l.market_side IN ('over', 'under')
		  AND l.event_id IS NOT NULL
		  AND %s
		GROUP BY 1, 2`, scope), id)
	if err != nil {
		return nil, fmt.Errorf("corner bet counts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var side, status string
		var count int
		if err := rows.Scan(&side, &status, &count); err != nil {
			return nil, err
		}
		target := &record.Over
		if side == "under" {
			target = &record.Under
		}
		target.Legs += count
		switch status {
		case "won":
			target.Won += count
		case "lost":
			target.Lost += count
		case "void":
			target.Void += count
		case "pending":
			target.Pending += count
		default:
			target.Unknown += count
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Money is only meaningful when the slip has one leg. A five-fold's profit
	// cannot be attributed to this match.
	if err := s.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*), COALESCE(SUM(b.stake), 0), COALESCE(SUM(b.profit_loss), 0)
		FROM personal_bets b
		JOIN personal_bet_legs l ON l.personal_bet_id = b.id
		WHERE l.market_family = 'corners_total'
		  AND l.event_id IS NOT NULL
		  AND %s
		  AND (SELECT count(*) FROM personal_bet_legs x WHERE x.personal_bet_id = b.id) = 1`,
		scope), id).Scan(&record.SingleBet.Bets, &record.SingleBet.Stake, &record.SingleBet.ProfitLoss); err != nil {
		return nil, fmt.Errorf("corner bet money: %w", err)
	}

	if record.Over.Legs == 0 && record.Under.Legs == 0 {
		return record, nil
	}
	return record, nil
}
