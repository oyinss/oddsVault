package api

import (
	"context"
	"fmt"
	"sort"

	"github.com/oyins/oddsvault/backend/internal/normalize"
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
	Recent    []cornerBetLeg `json:"recent"`
}

// cornerBetLeg is one recorded line, shown so the line can be reused.
type cornerBetLeg struct {
	Side   string   `json:"side"`
	Line   *float64 `json:"line"`
	Odds   *float64 `json:"odds"`
	Status string   `json:"status"`
	Date   *string  `json:"date"`
	Home   string   `json:"home,omitempty"`
	Away   string   `json:"away,omitempty"`
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
	LeagueLegs int `json:"league_legs"`
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

// placedByString names the bettor behind a journal leg. An unlabelled bet is
// reported as unknown rather than guessed from its stake.
func placedByString(value *string) string {
	if value == nil || *value == "" {
		return "unknown"
	}
	return *value
}

// matchBetHistory is a compact view of the journal for one board row. It exists
// so the board can answer "have I played this league or these teams before?"
// without opening the match.
type matchBetHistory struct {
	Legs      int  `json:"legs"`
	Won       int  `json:"won"`
	Lost      int  `json:"lost"`
	Void      int  `json:"void"`
	Open      int  `json:"open"`
	Unsettled int  `json:"unsettled"`
	OverWon   int  `json:"over_won"`
	OverLost  int  `json:"over_lost"`
	UnderWon  int  `json:"under_won"`
	UnderLost int  `json:"under_lost"`
	League    bool `json:"league"`
	Teams     bool `json:"teams"`
	// Recent carries the last five corner legs that touched this league or
	// either team, newest first, so the board can show the selection itself
	// instead of only a win-loss tally.
	Recent []matchBetRecent `json:"recent,omitempty"`
}

// matchBetRecent is one played corner leg as the board renders it.
type matchBetRecent struct {
	Side   string   `json:"side"`
	Line   *float64 `json:"line"`
	Odds   *float64 `json:"odds"`
	Status string   `json:"status"`
	Date   *string  `json:"date"`
	Scope  string   `json:"scope"`
	// By names who played it: emife, oyins or agent from the journal, paper for
	// a pick recorded on this board.
	By string `json:"by"`
	// The settled match, so a row can be read against what actually happened.
	// Total is the pair of published corner counts; the halves come from our
	// snapshots and stay empty when the match was not tracked past the break.
	Total      *int `json:"total_corners,omitempty"`
	FirstHalf  *int `json:"first_half_corners,omitempty"`
	SecondHalf *int `json:"second_half_corners,omitempty"`
}

type journalLegRow struct {
	side          string
	status        string
	competitionID int64
	homeID        int64
	awayID        int64
	line          *float64
	odds          *float64
	placedAt      *string
	placedBy      *string
	eventID       int64
	total         *int
	firstHalf     *int
}

// attachMatchBetHistory fills the compact record for every board row. One
// journal read is shared across all matches; a leg is counted once per match
// even when both the league and a team match.
func (s *Server) attachMatchBetHistory(ctx context.Context, matches []liveCornerMatch) {
	if len(matches) == 0 {
		return
	}
	rows, err := s.pool.Query(ctx, `
		SELECT l.market_side, l.leg_status, e.competition_id, hp.team_id, ap.team_id,
		       l.market_line, l.odds_taken, to_char(e.start_date, 'YYYY-MM-DD'),
		       pb.placed_by, e.id,
		       CASE WHEN fin.n = 2 THEN fin.total END,
		       fh.first_half
		FROM personal_bet_legs l
		JOIN personal_bets pb ON pb.id = l.personal_bet_id
		JOIN events e ON e.id = l.event_id
		LEFT JOIN LATERAL (
			SELECT count(s.corners) AS n, sum(s.corners)::int AS total
			FROM team_match_stats s
			WHERE s.event_id = e.id AND s.corners IS NOT NULL
		) fin ON true
		LEFT JOIN LATERAL (
			SELECT cs.total_corners AS first_half
			FROM corner_snapshots cs
			WHERE cs.event_id = e.id
			  AND cs.minute IS NOT NULL
			  AND cs.minute <= 45
			  AND EXISTS (
			      SELECT 1 FROM corner_snapshots c2
			      WHERE c2.event_id = e.id AND c2.minute > 45
			  )
			ORDER BY cs.minute DESC, cs.captured_at DESC
			LIMIT 1
		) fh ON true
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		WHERE l.market_family = 'corners_total'
		  AND l.market_side IN ('over', 'under')
		ORDER BY e.start_date DESC, l.id DESC`)
	if err != nil {
		s.log.ErrorContext(ctx, "board bet history", "error", err.Error())
		return
	}
	defer rows.Close()

	var legs []journalLegRow
	for rows.Next() {
		var leg journalLegRow
		if err := rows.Scan(&leg.side, &leg.status, &leg.competitionID, &leg.homeID, &leg.awayID,
			&leg.line, &leg.odds, &leg.placedAt, &leg.placedBy,
			&leg.eventID, &leg.total, &leg.firstHalf); err != nil {
			s.log.ErrorContext(ctx, "scan board bet leg", "error", err.Error())
			return
		}
		legs = append(legs, leg)
	}
	if err := rows.Err(); err != nil {
		s.log.ErrorContext(ctx, "iterate board bet legs", "error", err.Error())
		return
	}
	leagueOnly, err := s.leagueOnlyCornerLegs(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "board league-only bet history", "error", err.Error())
	}

	paperByEvent := s.paperPicksForBoard(ctx, matches)

	if len(legs) == 0 && len(leagueOnly) == 0 && len(paperByEvent) == 0 {
		return
	}

	for index := range matches {
		match := &matches[index]
		history := matchBetHistory{}
		for _, leg := range leagueOnly[match.CompetitionID] {
			history.Legs++
			history.League = true
			recent := matchBetRecent{
				Side:   leg.side,
				Line:   leg.line,
				Odds:   leg.odds,
				Status: leg.status,
				Date:   leg.placedAt,
				Scope:  "league",
				By:     placedByString(leg.placedBy),
			}
			recent.applyResult(leg.total, leg.firstHalf)
			history.Recent = append(history.Recent, recent)
			switch leg.status {
			case "won":
				history.Won++
			case "lost":
				history.Lost++
			case "void":
				history.Void++
			case "pending":
				history.Open++
			default:
				history.Unsettled++
			}
			if leg.side == "over" {
				if leg.status == "won" {
					history.OverWon++
				}
				if leg.status == "lost" {
					history.OverLost++
				}
			} else {
				if leg.status == "won" {
					history.UnderWon++
				}
				if leg.status == "lost" {
					history.UnderLost++
				}
			}
		}
		for _, leg := range legs {
			inLeague := leg.competitionID == match.CompetitionID
			inTeams := leg.homeID == match.Home.ID || leg.awayID == match.Home.ID ||
				leg.homeID == match.Away.ID || leg.awayID == match.Away.ID
			if !inLeague && !inTeams {
				continue
			}
			history.Legs++
			history.League = history.League || inLeague
			history.Teams = history.Teams || inTeams
			scope := "team"
			if inLeague {
				scope = "league"
			}
			recent := matchBetRecent{
				Side:   leg.side,
				Line:   leg.line,
				Odds:   leg.odds,
				Status: leg.status,
				Date:   leg.placedAt,
				Scope:  scope,
				By:     placedByString(leg.placedBy),
			}
			recent.applyResult(leg.total, leg.firstHalf)
			history.Recent = append(history.Recent, recent)
			switch leg.status {
			case "won":
				history.Won++
			case "lost":
				history.Lost++
			case "void":
				history.Void++
			case "pending":
				history.Open++
			default:
				history.Unsettled++
			}
			if leg.side == "over" {
				if leg.status == "won" {
					history.OverWon++
				}
				if leg.status == "lost" {
					history.OverLost++
				}
			} else {
				if leg.status == "won" {
					history.UnderWon++
				}
				if leg.status == "lost" {
					history.UnderLost++
				}
			}
		}
		for _, pick := range paperByEvent[match.EventID] {
			history.Recent = append(history.Recent, pick)
		}
		if history.Legs > 0 || len(history.Recent) > 0 {
			// Newest first, then keep the five the card can show.
			sort.SliceStable(history.Recent, func(i, j int) bool {
				left, right := "", ""
				if history.Recent[i].Date != nil {
					left = *history.Recent[i].Date
				}
				if history.Recent[j].Date != nil {
					right = *history.Recent[j].Date
				}
				return left > right
			})
			if len(history.Recent) > 5 {
				history.Recent = history.Recent[:5]
			}
			match.BetHistory = &history
		}
	}
}

// applyResult attaches the settled match to a leg. The second half is the total
// minus the first half, and only appears when both numbers are real.
func (r *matchBetRecent) applyResult(total, firstHalf *int) {
	if total == nil {
		return
	}
	r.Total = total
	if firstHalf == nil {
		return
	}
	second := *total - *firstHalf
	if second < 0 {
		return
	}
	r.FirstHalf = firstHalf
	r.SecondHalf = &second
}

// paperPicksForBoard returns the picks already recorded on these live matches,
// so one list can show every corner leg the user has on a card: journal legs by
// bettor and paper picks by name.
func (s *Server) paperPicksForBoard(ctx context.Context, matches []liveCornerMatch) map[int64][]matchBetRecent {
	ids := make([]int64, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.EventID)
	}
	if len(ids) == 0 {
		return nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT p.event_id, p.side, p.line, p.odds, p.status,
		       to_char(p.created_at, 'YYYY-MM-DD'),
		       CASE WHEN fin.n = 2 THEN fin.total END,
		       fh.first_half
		FROM paper_picks p
		LEFT JOIN LATERAL (
			SELECT count(s.corners) AS n, sum(s.corners)::int AS total
			FROM team_match_stats s
			WHERE s.event_id = p.event_id AND s.corners IS NOT NULL
		) fin ON true
		LEFT JOIN LATERAL (
			SELECT cs.total_corners AS first_half
			FROM corner_snapshots cs
			WHERE cs.event_id = p.event_id
			  AND cs.minute IS NOT NULL
			  AND cs.minute <= 45
			  AND EXISTS (
			      SELECT 1 FROM corner_snapshots c2
			      WHERE c2.event_id = p.event_id AND c2.minute > 45
			  )
			ORDER BY cs.minute DESC, cs.captured_at DESC
			LIMIT 1
		) fh ON true
		WHERE p.event_id = ANY($1::bigint[])`, ids)
	if err != nil {
		s.log.ErrorContext(ctx, "board paper pick history", "error", err.Error())
		return nil
	}
	defer rows.Close()

	out := make(map[int64][]matchBetRecent, len(matches))
	for rows.Next() {
		var eventID int64
		var item matchBetRecent
		var total, firstHalf *int
		if err := rows.Scan(&eventID, &item.Side, &item.Line, &item.Odds,
			&item.Status, &item.Date, &total, &firstHalf); err != nil {
			s.log.ErrorContext(ctx, "scan board paper pick history", "error", err.Error())
			return out
		}
		item.Scope = "match"
		item.By = "paper"
		item.applyResult(total, firstHalf)
		out[eventID] = append(out[eventID], item)
	}
	return out
}

// leagueOnlyCornerLegs returns journal legs whose fixture never resolved but
// whose competition text matches exactly one competition by name or slug. This
// is identity, not a guess: a near name is not accepted, and a key shared by
// two competitions is dropped.
func (s *Server) leagueOnlyCornerLegs(ctx context.Context) (map[int64][]journalLegRow, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, name, slug FROM competitions`)
	if err != nil {
		return nil, fmt.Errorf("list competitions for journal: %w", err)
	}
	defer rows.Close()

	owners := make(map[string]map[int64]bool, 512)
	for rows.Next() {
		var id int64
		var name, slug string
		if err := rows.Scan(&id, &name, &slug); err != nil {
			return nil, err
		}
		for _, candidate := range []string{name, slug} {
			if candidate == "" {
				continue
			}
			_, key := normalize.Resolve(candidate)
			if key == "" {
				continue
			}
			if owners[key] == nil {
				owners[key] = make(map[int64]bool, 1)
			}
			owners[key][id] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	unique := make(map[string]int64, len(owners))
	for key, ids := range owners {
		if len(ids) == 1 {
			for id := range ids {
				unique[key] = id
			}
		}
	}

	legRows, err := s.pool.Query(ctx, `
		SELECT l.market_side, l.leg_status, l.competition_raw,
		       l.market_line, l.odds_taken, to_char(pb.placed_at, 'YYYY-MM-DD'),
		       pb.placed_by
		FROM personal_bet_legs l
		JOIN personal_bets pb ON pb.id = l.personal_bet_id
		WHERE l.market_family = 'corners_total'
		  AND l.event_id IS NULL
		  AND l.competition_raw IS NOT NULL
		  AND l.market_side IN ('over', 'under')`)
	if err != nil {
		return nil, fmt.Errorf("list league-only journal legs: %w", err)
	}
	defer legRows.Close()

	out := make(map[int64][]journalLegRow)
	for legRows.Next() {
		var side, status, raw string
		var line, odds *float64
		var placedAt, placedBy *string
		if err := legRows.Scan(&side, &status, &raw, &line, &odds, &placedAt, &placedBy); err != nil {
			return nil, err
		}
		_, key := normalize.Resolve(raw)
		competitionID, ok := unique[key]
		if !ok {
			continue
		}
		out[competitionID] = append(out[competitionID], journalLegRow{
			side:          side,
			status:        status,
			competitionID: competitionID,
			line:          line,
			odds:          odds,
			placedAt:      placedAt,
			placedBy:      placedBy,
		})
	}
	return out, legRows.Err()
}

// addLeagueLeg folds one league-only leg into a detail record.
func addLeagueLeg(record *cornerBetRecord, leg journalLegRow) {
	if record == nil {
		return
	}
	target := &record.Over
	if leg.side == "under" {
		target = &record.Under
	}
	target.Legs++
	switch leg.status {
	case "won":
		target.Won++
	case "lost":
		target.Lost++
	case "void":
		target.Void++
	case "pending":
		target.Pending++
	default:
		target.Unknown++
	}
}

// recentCornerLegs lists the lines behind one scope so the detail page can show
// what was actually played, newest first.
func (s *Server) recentCornerLegs(ctx context.Context, scope string, id int64, limit int) ([]cornerBetLeg, error) {
	rows, err := s.pool.Query(ctx, fmt.Sprintf(`
		SELECT l.market_side, l.market_line, l.odds_taken, l.leg_status,
		       to_char(e.start_date, 'YYYY-MM-DD'), h.name, a.name
		FROM personal_bet_legs l
		JOIN events e ON e.id = l.event_id
		JOIN event_participants hp ON hp.event_id = e.id AND hp.role = 'home'
		JOIN teams h ON h.id = hp.team_id
		JOIN event_participants ap ON ap.event_id = e.id AND ap.role = 'away'
		JOIN teams a ON a.id = ap.team_id
		WHERE l.market_family = 'corners_total'
		  AND l.market_side IN ('over', 'under')
		  AND %s
		ORDER BY e.start_date DESC, l.id DESC
		LIMIT $2`, scope), id, limit)
	if err != nil {
		return nil, fmt.Errorf("recent corner legs: %w", err)
	}
	defer rows.Close()

	var out []cornerBetLeg
	for rows.Next() {
		var item cornerBetLeg
		var date *string
		if err := rows.Scan(&item.Side, &item.Line, &item.Odds, &item.Status,
			&date, &item.Home, &item.Away); err != nil {
			return nil, err
		}
		item.Date = date
		out = append(out, item)
	}
	return out, rows.Err()
}

func leagueLegToRecent(leg journalLegRow) cornerBetLeg {
	return cornerBetLeg{
		Side:   leg.side,
		Line:   leg.line,
		Odds:   leg.odds,
		Status: leg.status,
		Date:   leg.placedAt,
	}
}
