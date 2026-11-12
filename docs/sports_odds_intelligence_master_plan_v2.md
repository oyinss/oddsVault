# Sports Odds Intelligence Platform — Master Build Plan v2

> **Purpose:** Consolidated implementation plan for the sports odds intelligence platform.
> **Core principle:** Preserve historical truth. Never fabricate missing sports, odds, result, or settlement data.

---

## 1. Product Goal

Build a sports intelligence platform that continuously captures real bookmaker odds, stores every historical movement, shows live scores, settles supported selections after events finish, and turns those records into searchable analytics.

The platform must let us study:

- what odds were available at a specific time;
- which selections eventually won, lost, pushed, voided, or could not be graded;
- how odds moved before kickoff;
- how different bookmakers priced the same market;
- what the user actually played;
- what a strategy/model selected before kickoff;
- how different bet styles and strategy versions performed historically.

The product should eventually support AI/agents for explanation and analysis, but deterministic code and database queries remain the source of truth for scores, odds, settlement, statistics, and historical percentages.

---

## 2. Final Technology Stack

### Web frontend

```text
Next.js App Router
React
TypeScript
Tailwind CSS
Recharts
```

Use **React for the website**. React Native is not part of the initial web build.

If a native Android/iOS app is added later, use:

```text
React Native + Expo
```

and reuse the same Go API.

### Backend

```text
Go
```

Go is responsible for:

- REST API;
- sports-provider ingestion;
- bookmaker collectors;
- normalization;
- live-score synchronization;
- result ingestion;
- settlement;
- background jobs;
- analytics;
- strategy execution.

Do not replace the main backend with Zig, Python, Node.js, or TypeScript unless a concrete requirement cannot reasonably be implemented in Go.

### Database

```text
PostgreSQL
+
TimescaleDB
```

PostgreSQL is the source of truth.

TimescaleDB is used for high-volume append-only time-series data, especially odds snapshots.

### Optional infrastructure

Add only when justified by measured need:

```text
Redis
Object storage
Server-Sent Events
WebSockets
```

Do not add infrastructure just because it is common in large systems.

---

## 3. Non-Negotiable Data Rules

### 3.1 No mock application data

Never insert fake:

- games;
- teams;
- players;
- bookmakers;
- odds;
- results;
- bets;
- model picks;
- analytics records.

into the real development or production database.

Tests must use:

- isolated unit-test files;
- disposable PostgreSQL/TimescaleDB databases;
- temporary schemas;
- transaction rollback where appropriate.

### 3.2 Missing data stays missing

If a provider does not supply a value, use:

```text
NULL
```

or an explicit state such as:

```text
needs_data
not_gradable
pending
unverified
unsupported
```

Never fabricate a score, stat, odds value, player identity, market outcome, or timeline event.

### 3.3 Zero is not the same as missing

```text
0
```

means the provider explicitly reported zero or a complete verified dataset proves zero.

```text
NULL
```

means unavailable or unknown.

### 3.4 Provider values and calculated values stay separate

Example:

```text
provider_final_score = 3-1
calculated_market_result = hit
historical_hit_rate = 58.3%
```

These belong to separate logical fields/tables.

### 3.5 Finished results are terminal

Once an event is verified as finished:

- a lagging feed may not revert it to scheduled/live;
- missing later data may not blank the final score;
- result corrections require an explicit versioned correction/audit trail.

### 3.6 Odds snapshots are immutable

If odds move:

```text
1.92 → 1.88 → 1.81 → 1.76
```

all observations remain stored.

Never overwrite the first price with the newest price.

### 3.7 Frozen model history

A model/strategy recommendation must be generated using only data available at that timestamp.

Never create a historical “what I should have played” slip after the result is known.

---

## 4. Sports Coverage

The normalized event system must be multi-sport from the beginning even if implementation is phased.

Priority:

```text
1. Football
2. Basketball
3. American Football
4. Tennis
5. Golf
```

### Football examples

```text
Premier League
La Liga
Serie A
Bundesliga
Ligue 1
Champions League
Europa League
Conference League
International competitions
Other domestic leagues
```

### Basketball examples

```text
NBA
WNBA
NCAA
EuroLeague
Other leagues
```

### American Football

```text
NFL
NCAA
```

### Tennis

```text
ATP
WTA
Grand Slams
Other tournaments
```

### Golf

```text
PGA Tour
DP World Tour
LIV Golf
Majors
```

Golf should not delay the rest of the platform if reliable free live/results data is not available.

---

## 5. Data Source Strategy

Use separate responsibilities.

### Sports-data providers supply

- fixtures;
- schedules;
- live scores;
- final scores;
- status;
- standings;
- lineups where available;
- team statistics;
- player statistics;
- timelines/events where available.

### Bookmaker collectors supply

- bookmaker-specific events;
- markets;
- selections;
- decimal odds;
- availability/suspension state;
- exact capture timestamp.

### Our database supplies long-term history

The platform should not repeatedly query third-party APIs for old records after those records have already been imported.

Flow:

```text
Provider / Bookmaker
        ↓
Go ingestion layer
        ↓
PostgreSQL + TimescaleDB
        ↓
Go API
        ↓
Next.js web application
```

---

## 6. Provider Abstraction

Provider-specific response shapes stop at the adapter boundary.

Suggested Go interfaces:

```go
type FixtureProvider interface {
    SyncFixtures(ctx context.Context, req FixtureSyncRequest) ([]NormalizedEvent, error)
}

type LiveScoreProvider interface {
    SyncLive(ctx context.Context, req LiveSyncRequest) ([]NormalizedLiveState, error)
}

type ResultProvider interface {
    SyncResults(ctx context.Context, req ResultSyncRequest) ([]NormalizedResult, error)
}
```

Optional interfaces:

```text
TeamStatsProvider
PlayerStatsProvider
TimelineProvider
LineupProvider
StandingsProvider
InjuryProvider
```

The database and frontend should understand normalized internal types, not provider-specific JSON.

---

## 7. Shared Provider HTTP Client

Retain the strongest behavior from the former project.

Every provider request should use one common Go client with:

```text
30-second timeout
maximum 4 attempts
exponential backoff
Retry-After support
retry on 429
retry on temporary 5xx
structured logs
request IDs
cache integration
quota tracking
optional force refresh
```

Do not retry normal validation/authentication 4xx errors blindly.

A provider outage must never delete previously imported rows.

---

## 8. Provider Response Cache

Keep a shared cache layer.

Concept:

```text
request
   ↓
cache lookup
   ↓
fresh? ── yes ──► cached response
   │
   no
   ↓
HTTP request
   ↓
save response
   ↓
normalize/import
```

Cache metadata:

```text
provider
request_key
request_hash
status
fetched_at
expires_at
etag
last_modified
```

Completed historical responses can be reused indefinitely unless forced or a correction is required.

---

## 9. Free API Quota Management

Because the project starts with free APIs, quota management is a core feature.

Store observations such as:

```text
provider
daily_limit
daily_remaining
minute_limit
minute_remaining
observed_at
```

### Highest priority requests

```text
live events connected to stored odds/bets
final-result confirmation
games starting soon
unsettled personal bets
unsettled model picks
```

### Medium priority

```text
today's fixtures
current-season statistics
standings
```

### Low priority

```text
historical backfill
secondary metadata
older competitions
```

Prefer endpoints that return many events in one request rather than one request per match.

---

## 10. Import Run Logging

Every ingestion operation should be logged.

Table:

```text
import_runs
```

Fields:

```text
id
provider
job_type
started_at
finished_at
status
records_received
records_inserted
records_updated
records_skipped
error_message
metadata jsonb
```

This becomes part of the admin data-health interface.

---

## 11. Core Sports Data Model

Use a sport-neutral event model.

### sports

```text
id
name
slug
active
```

### competitions

```text
id
sport_id
country_id nullable
name
slug
type
active
```

### seasons

```text
id
competition_id
name
start_date nullable
end_date nullable
is_current
```

Only one current season per competition.

### teams

```text
id
name
short_name
slug
country_id
```

### team_provider_ids

```text
team_id
provider
provider_team_id
```

Unique:

```text
(provider, provider_team_id)
```

### players

```text
id
name
date_of_birth nullable
nationality nullable
```

### player_provider_ids

```text
player_id
provider
provider_player_id
```

### events

```text
id
sport_id
competition_id
season_id nullable
start_time
status
venue_id nullable
created_at
updated_at
```

### event_participants

Supports different sport shapes.

Football:

```text
home → Arsenal
away → Chelsea
```

Tennis:

```text
player_1
player_2
```

Golf:

```text
multiple competitors
```

### event_provider_ids

```text
event_id
provider
provider_event_id
observed_start_time
observed_status
linked_at
```

---

## 12. Event Identity Matching

Never guess when event identity is ambiguous.

Matching precedence:

```text
1. exact provider + provider_event_id
2. known alias/source mapping
3. sport + competition + participants + sufficiently close start time
4. unique competition/season/participant candidate
5. unresolved/manual review
```

If multiple candidates exist:

```text
identity_status = unresolved
```

Store matching evidence for audit.

---

## 13. Bookmakers

Initial target bookmakers:

```text
SportyBet
Betano
1xBet Nigeria
LivescoreBet
```

Use one generic bookmaker model.

Table:

```text
bookmakers
```

Fields:

```text
id
name
slug
country_scope nullable
base_url nullable
active
```

Do not create separate odds tables per bookmaker.

---

## 14. Bookmaker Collectors

Each bookmaker gets its own isolated Go collector.

Suggested structure:

```text
backend/internal/bookmakers/
    sportybet/
    betano/
    onexbet/
    livescorebet/
```

A collector may use:

```text
documented API
public structured JSON/XHR responses
HTML parsing where permitted and technically necessary
```

Do not mix bookmaker-specific parsing logic into the core odds model.

---

## 15. Bookmaker Events

Table:

```text
bookmaker_events
```

Fields:

```text
id
bookmaker_id
provider_event_id
raw_name
scheduled_at
matched_event_id nullable
identity_status
first_seen_at
last_seen_at
```

---

## 16. Alias Mapping

Different providers/bookmakers may name the same entity differently.

Examples:

```text
Man United
Man Utd
Manchester United
```

Use mapping tables:

```text
team_aliases
competition_aliases
participant_aliases
bookmaker_market_aliases
```

Aliases resolve to canonical records.

---

## 17. Market Normalization

Raw bookmaker labels may differ.

Example:

```text
Over 2.5
Total Goals Over 2.5
Total Over (2.5)
```

Normalize to:

```text
market_family = total_goals
selection = over
line = 2.5
period = full_game
```

Always retain raw labels.

Store both:

```text
raw_market_name
raw_selection_name
normalized_market_type
normalized_selection_type
```

---

## 18. Canonical Market Model

Suggested tables:

```text
market_types
markets
market_selections
```

Normalized dimensions:

```text
sport
period
subject_type
subject_id
market_family
selection
line
handicap
side
qualifier
```

Example:

```text
sport = football
period = full_match
subject_type = match
market_family = total_goals
selection = over
line = 2.5
```

---

## 19. Snapshot Batches

Each bookmaker collection run creates a batch.

Table:

```text
snapshot_batches
```

Fields:

```text
id
bookmaker_id
started_at
completed_at
status
event_count
market_count
offer_count
collector_version
error_summary nullable
```

This lets us reconstruct what was available during a specific capture run.

---

## 20. Odds Snapshots

Use a TimescaleDB hypertable.

Table:

```text
odds_snapshots
```

Fields:

```text
captured_at
snapshot_batch_id
bookmaker_id
event_id nullable
bookmaker_event_id
market_id
selection_id
decimal_odds
availability_status
raw_price nullable
source_hash
```

Insert new observations.

Never overwrite old prices.

---

## 21. Snapshot Frequency

Suggested initial schedule:

```text
>24h before start     every 30–60 minutes
24h–6h                every 15 minutes
6h–1h                 every 5 minutes
<1h                   every 1–2 minutes when practical
live                  separate policy
```

Frequency should be configurable per bookmaker and sport.

Add jitter/backoff to avoid synchronized bursts.

---

## 22. Opening, Current, Closing, High, Low

These are derived values from raw snapshots.

Expose:

```text
opening_odds
current_odds
closing_odds
highest_odds
lowest_odds
first_seen_at
last_seen_at
```

Do not use them as a replacement for raw history.

TimescaleDB continuous aggregates may be used for common calculations.

---

## 23. Cross-Bookmaker Comparison

For one normalized market, compare:

```text
SportyBet
Betano
1xBet
LivescoreBet
```

Show:

- captured price;
- capture age;
- opening price;
- movement;
- best captured price;
- average/median where useful.

Always show freshness such as:

```text
Captured 2m ago
```

Never imply an old price is still available.

---

## 24. Live Scores

Live score is a first-class feature.

The UI should show:

- score;
- game clock/minute;
- quarter/period/set;
- event status;
- key timeline events;
- verified live stats where available;
- team/player imagery;
- related stored odds.

Flow:

```text
Sports API
    ↓
Go live worker
    ↓
PostgreSQL
    ↓
Go API
    ↓
Next.js
```

---

## 25. Live State Model

Tables:

```text
event_live_states
event_score_components
event_timeline_sources
event_timeline_events
```

Sport-specific examples:

### Football

```text
home_score
away_score
minute
stoppage_time
```

### Basketball

```text
Q1
Q2
Q3
Q4
OT
clock
```

### Tennis

```text
sets
games
current_set
server
```

### American Football

```text
quarter
clock
score
```

---

## 26. Final Results

Final result data stays independent from odds.

Tables:

```text
event_results
team_match_stats
player_match_stats
event_result_timelines
```

`NULL` means unavailable.

Explicit zero means zero.

---

## 27. Historical Ingestion

Preserve the strongest behavior of the former project.

Flow:

```text
Provider
   ↓
shared cached HTTP client
   ↓
provider adapter
   ↓
normalized records
   ↓
validation
   ↓
transactional idempotent importer
   ↓
PostgreSQL
```

Requirements:

- idempotent reruns;
- no duplicate teams/events/stats;
- logical transaction boundaries;
- provider failures isolated;
- existing verified data retained;
- nullable columns preserved;
- source timestamps retained.

---

## 28. Raw Provider Payloads

Store selectively for:

- debugging;
- parser regression;
- reprocessing after adapter fixes;
- settlement evidence;
- data disputes.

Table:

```text
raw_provider_payloads
```

Fields:

```text
provider
entity_type
provider_entity_id
captured_at
payload
payload_hash
schema_version
```

Frontend business logic must never depend directly on raw payload format.

---

## 29. Settlement Engine

Settlement is deterministic, sport-specific, and versioned.

Suggested structure:

```text
backend/internal/settlement/
    engine.go
    rules/
        football/
        basketball/
        americanfootball/
        tennis/
        golf/
```

Main settlement statuses:

```text
hit
miss
push
void
needs_data
pending
not_gradable
```

Every settlement includes:

```text
actual_value
result_detail
reason_code
data_source
source_updated_at
evaluated_at
rule_version
evaluation_version
```

---

## 30. Settlement Preconditions

Do not settle because enough time has passed.

Settlement requires:

```text
event.status = finished
```

plus whatever verified data the specific market requires.

Examples:

### Match Result

Requires verified final score.

### Player Shots on Target

Requires verified player stat feed.

### First Goalscorer

Requires complete official goal timeline and resolvable scorer identity.

### First-Half Corners

Requires verified period-specific corner information.

If required data is unavailable:

```text
needs_data
```

---

## 31. Settlement Reason Codes

Examples:

```text
final_score_pending
stat_unavailable
player_feed_incomplete
timeline_unavailable
timeline_goal_mismatch
timeline_team_unresolved
timeline_identity_unresolved
identity_unresolved
unsupported_market
unsupported_selection
rule_unverified
mapping_not_verified
period_data_unavailable
no_action
bookmaker_rule_unverified
```

These should be searchable in admin.

---

## 32. Football Settlement Rules

Initial deterministic support:

```text
1X2
Double Chance
Draw No Bet
Correct Score
BTTS
Over/Under Goals
Team Totals
Handicap where verified
Clean Sheet
Anytime Goalscorer
Score or Assist
Player Shots
Player Shots on Target
Player Passes
Player Tackles
Goalkeeper Saves
Corners
Team Corners
Period markets with verified period data
```

If a bookmaker-specific rule is not confirmed:

```text
not_gradable
```

---

## 33. Basketball Settlement Rules

Initial support:

```text
Moneyline
Spread
Game Total
Team Total
Half/Quarter markets
Player Points
Rebounds
Assists
Three-pointers
Steals
Blocks
Combined props
```

Overtime inclusion must be explicit in the normalized market rule.

---

## 34. American Football Settlement Rules

Initial support:

```text
Moneyline
Spread
Game Total
Team Total
Quarter/Half markets
Passing Yards
Rushing Yards
Receiving Yards
Touchdowns
Receptions
Field Goals
```

Only verified rule variants are settled.

---

## 35. Tennis Settlement Rules

Initial support:

```text
Match Winner
Set Winner
Total Games
Game Handicap
Set Handicap
Exact Set Score
```

Retirements, walkovers, and abandoned matches require explicit policies.

Do not assume bookmaker void rules.

---

## 36. Settlement Version History

Never silently overwrite a prior settlement.

Tables:

```text
selection_settlements
selection_settlement_versions
```

Example:

```text
evaluation_version 1
evaluation_version 2
```

The latest accepted version is current, but older versions remain auditable.

---

## 37. Snapshot History Page

This is a core learning page.

Filters:

```text
date
time
sport
competition
bookmaker
market
odds range
settlement
strategy/model association
```

Settlement filters:

```text
All
Hit
Miss
Push
Void
Needs Data
Pending
```

Clicking a snapshot should show:

- original odds;
- bookmaker;
- event;
- capture timestamp;
- final result;
- settlement;
- reason code;
- full line movement;
- other bookmaker prices;
- relevant verified statistics;
- source/freshness.

---

## 38. Historical Analytics

Support queries such as:

```text
Football
Premier League
Over 2.5
Odds 1.70–1.79
```

Return:

```text
settled sample count
hits
misses
pushes
voids
historical hit rate
average odds
median odds
average implied probability
historical minus implied difference
```

Never count `needs_data` as a miss.

Every percentage must display sample size.

Prefer:

```text
58.4% (2,842 settled samples)
```

not simply:

```text
58.4%
```

---

## 39. Odds Buckets

Support configurable odds ranges.

Examples:

```text
1.20–1.29
1.30–1.39
```

or finer:

```text
1.70–1.74
1.75–1.79
```

Possible bucket sizes:

```text
0.10
0.05
custom
```

---

## 40. Line Movement Analytics

Expose:

```text
opening odds
current odds
closing odds
high
low
absolute movement
percentage movement
movement direction
time of largest move
cross-bookmaker disagreement
```

Historical query example:

```text
Show Over 2.5 selections that opened >= 2.00
and closed below 1.80.
```

Then show how those selections historically settled.

This is descriptive history, not a guarantee of future probability.

---

## 41. Personal Bet Journal — My Bets

Track what was actually played.

Tables:

```text
personal_bets
personal_bet_legs
personal_bet_versions
```

Store:

```text
bookmaker
placed_at
stake
bet_type
combined_odds
selected legs
odds actually taken
potential return
final result
actual return
profit/loss
```

A later price must never replace the price actually taken.

---

## 42. Model Picks

This stores what the system selected **before kickoff**.

Tables:

```text
model_runs
model_bets
model_bet_legs
strategy_versions
```

Required fields:

```text
strategy_id
strategy_version
generated_at
data_cutoff_at
snapshot_batch_id
combined_odds
status
locked_at
```

The recommendation must be immutable after its cutoff/lock point.

---

## 43. Played vs Model Comparison

Dedicated learning view.

Example:

```text
ACTUAL BET               MODEL PICK

5 selections             5 selections
6.84 odds                5.91 odds
Lost                     Won
-₦5,000                  simulated +₦X
```

Then show:

```text
same selections
model-avoided selections
model-added selections
odds differences
historical indicators available at decision time
```

This is a review tool, not proof that the model was objectively correct.

---

## 44. Bet Style Engine

Initial strategy shapes:

```text
1 selection × 10 games
1 selection × 5 games
1 selection × 3 games

2 selections × 6 games
2 selections × 5 games
2 selections × 4 games
2 selections × 3 games

3–4 selections × 4 games
3–4 selections × 3 games
3–4 selections × 2 games
3–4 selections × 1 game
```

Bet structures:

```text
Single
Parlay
Bet Builder
```

Store strategy definitions in data/configuration.

Tables:

```text
strategy_definitions
strategy_versions
strategy_rules
```

Never edit an already-used strategy version.

Create a new version.

---

## 45. Candidate Ranking Engine

Keep ranking separate from bet construction.

Flow:

```text
available events/markets
        ↓
historical analytics
        ↓
candidate score/ranking
        ↓
strategy eligibility
        ↓
bet construction
        ↓
frozen model bet
```

Candidate metrics may include:

```text
historical sample size
historical hit rate
implied probability
historical-vs-implied difference
current odds
opening odds
line movement
competition history
team/player history
market history
bookmaker price comparison
recency weighting
data completeness
sample quality
```

Do not use an LLM for core arithmetic/statistical calculations.

Use Go/SQL.

---

## 46. Backtesting

Historical replay must reconstruct exactly what was known at a past timestamp.

Use:

```text
events available then
odds available then
snapshot timestamp
strategy version then
```

Backtests are read-only against production historical source tables.

They must not mutate odds/history.

Track:

```text
backtest_runs
strategy_version
data_cutoff
sample_count
results
```

Evaluate winners, losers, pushes, voids, and unavailable-data cases to avoid survivorship bias.

---

## 47. Main Website Navigation

Suggested navigation:

```text
Home
Live
Sports
Odds
Snapshots
Analytics
My Bets
Model Picks
Strategies
History
```

Admin-only:

```text
Data Health
Import Runs
Collectors
Provider Quota
Unmatched Events
Market Mappings
Settlement Review
```

---

## 48. Home Page

Suggested modules:

```text
Live Now
Today's Games
Sports Navigation
Major Leagues
Top Odds Movement
Latest Settled Snapshots
My Active Bets
Model Picks
Played vs Model
Quick Analytics
```

---

## 49. Sports-Website Visual Design

The product should feel like a sports website, not a plain database dashboard.

Use verified/legitimate assets where available:

```text
team crests
competition logos
player headshots
country flags
sport icons
bookmaker logos
venue imagery where useful
```

Examples:

```text
[Arsenal crest] Arsenal 2–1 Chelsea [Chelsea crest]
```

Player prop:

```text
[player headshot]
Bukayo Saka
Shots on Target O1.5
```

Images should improve recognition and scanning.

Do not add random decorative sports photos that do not correspond to the event/entity.

---

## 50. Media Asset System

Table:

```text
media_assets
```

Fields:

```text
id
entity_type
entity_id
asset_type
source_provider
source_url
storage_url
width
height
mime_type
fetched_at
license_metadata nullable
active
```

Asset types:

```text
team_logo
competition_logo
player_photo
bookmaker_logo
venue_photo
sport_icon
```

Fallback order:

```text
1. verified provider/official asset
2. stored team/competition asset
3. neutral initials/icon
4. generic sport icon
```

Use Next.js Image for optimized delivery.

---

## 51. Live Page

Filters:

```text
sport
competition
bookmaker availability
favorites
```

Event cards should show:

```text
logos/headshots
score
clock/status
key timeline events
selected markets
movement indicator
snapshot state
personal bet indicator
model pick indicator
```

---

## 52. Event Page

Header:

```text
competition logo
competition name
participant images
participants
score
status
start time
```

Tabs:

```text
Overview
Odds
Movement
Stats
Timeline
Snapshots
My Bets
Model Picks
History
```

---

## 53. Odds Page

Features:

```text
bookmaker comparison
best captured odds
opening odds
current odds
movement
last capture time
market filters
```

Always expose capture freshness.

---

## 54. My Bets Page

Views:

```text
Active
Settled
Won
Lost
Void
All
```

Show:

```text
bookmaker
stake
odds
legs
live progress
settlement
P/L
snapshot context
model comparison
```

---

## 55. Model Picks Page

Views:

```text
Upcoming
Live
Settled
By Strategy
By Version
```

Show:

```text
generation timestamp
data cutoff
strategy/version
snapshot batch
selections
original odds
settlement
comparison with actual bet
```

---

## 56. Strategy Performance Page

Metrics:

```text
total generated
fully settled
full-slip win rate
leg hit rate
average combined odds
simulated ROI
max losing sequence
sample size
date range
sport
competition
bookmaker
```

Any ROI simulation must state its staking assumption.

---

## 57. Search

Global search:

```text
teams
players
competitions
events
bookmakers
```

Include logos/headshots where available.

---

## 58. Admin — Data Health

### Import Runs

Show:

```text
provider
job
start
duration
received
inserted
updated
skipped
status
error
```

### Provider Quota

Show:

```text
daily limit
remaining
minute limit
remaining
last request
```

### Collector Health

Show:

```text
bookmaker
last successful capture
event count
market count
offer count
errors
```

### Unmatched Events

Allow manual review when identity cannot be verified.

### Market Mapping

Display unknown bookmaker market labels and allow verified mappings.

### Settlement Review

Filter:

```text
needs_data
not_gradable
unsupported
identity_unresolved
```

---

## 59. Collector Change Detection

Bookmaker sites change frequently.

Monitor:

```text
parse success rate
expected fields
event count anomalies
market count anomalies
unexpected empty result
schema fingerprint
```

If a collector suddenly returns zero events when events are expected:

```text
do not treat zero as a valid deletion
```

Raise a collector-health issue instead.

---

## 60. Suggested Database Domains

### Core

```text
sports
countries
competitions
seasons
teams
team_provider_ids
team_competitions
players
player_provider_ids
events
event_participants
event_provider_ids
venues
```

### Provider/Data Quality

```text
import_runs
provider_request_cache
provider_quota_observations
raw_provider_payloads
event_identity_reviews
```

### Results

```text
event_results
team_match_stats
player_match_stats
event_result_timelines
event_timeline_sources
event_timeline_events
```

### Bookmakers/Odds

```text
bookmakers
bookmaker_events
team_aliases
competition_aliases
bookmaker_market_aliases
snapshot_batches
market_types
markets
market_selections
odds_snapshots
```

### Settlement

```text
selection_settlements
selection_settlement_versions
settlement_rule_versions
```

### Personal/Model

```text
personal_bets
personal_bet_legs
personal_bet_versions
model_runs
model_bets
model_bet_legs
strategy_definitions
strategy_versions
strategy_rules
```

### Analytics

```text
backtest_runs
analytics_materializations
```

### Media

```text
media_assets
```

---

## 61. Indexing Strategy

Examples:

```text
events (sport_id, start_time)
events (competition_id, season_id, start_time)
event_provider_ids (provider, provider_event_id) UNIQUE
team_provider_ids (provider, provider_team_id) UNIQUE
player_provider_ids (provider, provider_player_id) UNIQUE
```

Odds indexes:

```text
(bookmaker_id, event_id, captured_at DESC)
(market_id, selection_id, captured_at DESC)
(event_id, captured_at DESC)
```

Settlement indexes:

```text
(selection_id, evaluated_at DESC)
(status)
(reason_code)
```

Use `EXPLAIN ANALYZE` before adding speculative indexes.

---

## 62. TimescaleDB Usage

Primary hypertable:

```text
odds_snapshots
```

Possible additional time-series:

```text
provider_quota_observations
live-state history if historical replay is retained
```

Use:

```text
hypertables
compression
continuous aggregates
```

Never create a retention policy that destroys required odds history.

---

## 63. Go API

Expose domain-oriented endpoints rather than raw database-table endpoints.

Examples:

```text
GET /api/v1/live
GET /api/v1/events
GET /api/v1/events/{id}
GET /api/v1/events/{id}/odds
GET /api/v1/events/{id}/movement
GET /api/v1/events/{id}/snapshots
GET /api/v1/snapshots
GET /api/v1/analytics/odds-ranges
GET /api/v1/analytics/markets
GET /api/v1/my-bets
GET /api/v1/model-picks
GET /api/v1/strategies
```

Admin:

```text
GET /api/v1/admin/import-runs
GET /api/v1/admin/provider-health
GET /api/v1/admin/unmatched-events
GET /api/v1/admin/unmapped-markets
POST /api/v1/admin/event-links
POST /api/v1/admin/market-mappings
```

---

## 64. Realtime Frontend Updates

Initial implementation:

```text
short polling against our Go API
```

Later if needed:

```text
Server-Sent Events
```

or:

```text
WebSockets
```

Never make each browser independently consume third-party provider quota.

---

## 65. Backend Commands/Processes

Recommended executables:

```text
cmd/api
cmd/sports-sync
cmd/bookmaker-collector
cmd/settlement-worker
cmd/analytics-worker
```

Keep one modular Go codebase initially.

Do not split into many microservices prematurely.

---

## 66. Repository Structure

```text
/
├── apps/
│   └── web/
│       ├── app/
│       ├── components/
│       ├── features/
│       ├── lib/
│       └── public/
│
├── backend/
│   ├── cmd/
│   │   ├── api/
│   │   ├── sports-sync/
│   │   ├── bookmaker-collector/
│   │   ├── settlement-worker/
│   │   └── analytics-worker/
│   │
│   ├── internal/
│   │   ├── api/
│   │   ├── database/
│   │   ├── providers/
│   │   ├── bookmakers/
│   │   ├── normalization/
│   │   ├── settlement/
│   │   ├── analytics/
│   │   ├── strategies/
│   │   ├── media/
│   │   └── jobs/
│   │
│   └── migrations/
│
├── docs/
└── deploy/
```

---

## 67. Background Jobs

Job types:

```text
sync_future_events
sync_live_events
sync_finished_results
sync_event_stats
capture_bookmaker_odds
settle_event
refresh_needs_data
refresh_media
build_analytics
```

Every job should be:

```text
idempotent where possible
retry-safe
observable
bounded
manually rerunnable
```

Redis is optional initially.

A Postgres-backed job table or internal scheduler is enough until measured load justifies more infrastructure.

---

## 68. Automatic Settlement Flow

```text
live provider sync
      ↓
event becomes finished
      ↓
persist verified final score
      ↓
fetch required final stats/timeline
      ↓
run completeness gates
      ↓
settle supported snapshot selections
      ↓
settle personal bets
      ↓
settle model picks
      ↓
refresh analytics
```

If a required stat feed is incomplete:

```text
needs_data
```

and a refresh worker can retry later.

---

## 69. Data Freshness

Expose freshness in the UI where useful.

Examples:

```text
score updated 22s ago
odds captured 3m ago
player stats updated 8m ago
```

This is especially important for live score and bookmaker odds.

---

## 70. Auditability

Critical records should carry:

```text
created_at
updated_at
source
source_updated_at
version
```

Decision records should also store:

```text
input_snapshot_id
rule_version
strategy_version
```

The goal is to reproduce why a result or recommendation existed.

---

## 71. Security

Requirements:

```text
third-party API keys server-side only
no secrets in Next.js client bundles
admin authentication
input validation in Go
parameterized SQL
public API rate limiting
secure session cookies
permission model
production secret management
```

---

## 72. User Accounts

Public odds/history pages should not require login unless necessary.

Account features may include:

```text
My Bets
favorites
saved filters
saved strategies
alerts
private model experiments
```

Keep public market history separated from private user records.

---

## 73. Testing

### Unit tests

Test:

```text
normalization
event matching
bookmaker parsing
market mapping
settlement rules
odds analytics
strategy constraints
```

### Integration tests

Use a disposable PostgreSQL/TimescaleDB instance.

Never write test rows into a real app database.

### Provider response fixtures

Where permitted, store sanitized real response samples as test files.

They are test inputs, not database seed data.

### Settlement regression tests

Every market rule should test:

```text
hit
miss
push
void
missing data
edge cases
```

---

## 74. Observability

Minimum metrics/logs:

```text
structured logs
job duration
job failures
provider request count
quota remaining
collector success rate
odds observations per run
settlement counts by result
settlement counts by reason
unmatched event count
unmapped market count
```

---

## 75. Backups

The historical odds dataset will become one of the platform's most valuable assets.

Required:

```text
automated PostgreSQL backups
offsite backup
point-in-time recovery when supported
restore testing
backup before risky migrations
```

Never keep the only copy of historical odds on one server.

---

## 76. Development Phases

### Phase 0 — Foundation

Build:

```text
monorepo
Next.js app
Go backend
PostgreSQL
TimescaleDB
migrations
logging
configuration/secrets
health endpoints
```

Exit condition:

```text
services boot cleanly
database migrations work
database contains only real configuration/reference records
```

### Phase 1 — Football Sports Data

Build:

```text
football provider adapter
competitions
seasons
teams/provider IDs
events
fixtures/results
live score
import runs
cache
quota monitoring
```

### Phase 2 — First Bookmaker Collector

Build:

```text
bookmaker events
event matching
normalized markets
snapshot batches
odds hypertable
snapshot history
```

Start with one bookmaker and prove the pipeline.

### Phase 3 — Football Settlement

Initial markets:

```text
1X2
Double Chance
Draw No Bet
BTTS
Over/Under 1.5
Over/Under 2.5
Over/Under 3.5
Correct Score
```

### Phase 4 — Sports Visual UI

Build:

```text
team crests
competition logos
sport icons
live page
event page
odds comparison
snapshot history
```

### Phase 5 — Remaining Bookmakers

Add:

```text
SportyBet
Betano
1xBet
LivescoreBet
```

Order may change depending on collection difficulty.

### Phase 6 — Analytics

Build:

```text
odds buckets
hit rates
sample sizes
movement analytics
opening/closing line
filters
```

### Phase 7 — My Bets

Build real personal bet tracking and settlement.

### Phase 8 — Strategies / Model Picks

Build:

```text
strategy definitions
strategy versions
candidate ranking
model runs
frozen picks
Played vs Model comparison
```

### Phase 9 — Basketball

Prioritize:

```text
NBA
WNBA
NCAA
```

### Phase 10 — NFL / NCAA Football

Add provider adapter and settlement rules.

### Phase 11 — Tennis

Add tennis event/result model and rules.

### Phase 12 — Golf

Only after a reliable source passes the required quality checks.

---

## 77. MVP Definition

Do not try to launch with every sport and every market.

A strong first MVP:

```text
Football
Premier League
La Liga
Champions League
2 bookmakers
live scores
real odds snapshots
automatic settlement
snapshot history
basic odds-range analytics
My Bets
Model Picks framework
sports-style visual UI
```

Initial markets:

```text
1X2
Double Chance
Draw No Bet
BTTS
Over 1.5
Over 2.5
Over 3.5
```

---

## 78. What Not to Build Too Early

Avoid early complexity such as:

```text
Kubernetes
Kafka
dozens of microservices
large Redis architecture without measured need
machine-learning pipelines before enough history exists
LLM-based numerical settlement
hundreds of props at once
full golf coverage without reliable data
```

---

## 79. AI / Agent Layer

Later, AI can:

```text
explain candidate rankings
summarize line movement
compare strategy versions
surface unusual historical patterns
answer natural-language history questions
review wins and losses
```

AI must not be the source of truth for:

```text
scores
odds
settlement
historical hit rates
numeric calculations
event identity
```

Those remain deterministic.

---

## 80. Native Mobile App Later

The product is web-first.

Initial frontend:

```text
Next.js + React + TypeScript + Tailwind
```

If a native app is added later:

```text
React Native + Expo
```

Both web and mobile use the same Go API and same backend/data model.

Do not build a second backend.

---

## 81. New Sport Quality Gate

A sport is fully supported only when the system can reliably:

1. identify events;
2. show schedules;
3. show live state;
4. detect final status;
5. match bookmaker events;
6. normalize key markets;
7. settle those markets deterministically;
8. handle sport-specific cancellations/exceptions;
9. show useful imagery;
10. generate historical analytics.

If those conditions are incomplete, mark the sport as partially supported.

---

## 82. New Market Quality Gate

Before enabling automatic settlement for a market:

1. raw bookmaker meaning is understood;
2. normalized mapping is verified;
3. required result data is identified;
4. settlement rule is formally defined;
5. push/void edge cases are defined;
6. bookmaker-specific variations are checked;
7. unit tests exist;
8. missing-data behavior is defined.

Otherwise use:

```text
not_gradable
```

or:

```text
needs_data
```

---

## 83. Final Architecture

```text
                         SPORTS DATA PROVIDERS
                                  │
                                  ▼
                         Go Provider Adapters
                                  │
                                  ▼
                         Normalized Sports Data
                                  │
                                  ▼
┌────────────────────┐     PostgreSQL      ┌──────────────────────┐
│ Bookmaker          │ ──► + TimescaleDB ◄─│ Results / Live Data  │
│ Collectors         │                     └──────────────────────┘
└────────────────────┘
          │
          ▼
  Immutable Odds History
          │
          ▼
   Settlement Engine
          │
          ├──────────────► Snapshot Results
          │
          ├──────────────► My Bets
          │
          └──────────────► Model Picks
                                  │
                                  ▼
                         Analytics / Strategies
                                  │
                                  ▼
                              Go API
                                  │
                                  ▼
                  Next.js + React + TypeScript
                                  │
                                  ▼
                      Sports Intelligence UI
```

---

## 84. Core Learning Loop

```text
collect real odds
      ↓
store immutable snapshots
      ↓
track live event
      ↓
store verified final data
      ↓
settle supported markets
      ↓
analyze wins/losses
      ↓
rank future candidates
      ↓
apply bet strategy
      ↓
freeze model pick
      ↓
compare model vs actual bet
      ↓
create improved strategy version
```

Never alter old historical recommendations to make a later strategy look better.

---

## 85. Final Principle

When choosing between convenience and preserving historical truth:

```text
preserve historical truth
```

The platform's most valuable long-term asset is:

```text
timestamped bookmaker odds
+
verified sporting outcomes
+
normalized markets
+
versioned settlements
+
actual personal bets
+
time-frozen model picks
+
strategy history
```

If that dataset remains trustworthy, the frontend, analytics, strategies, agents, and future models can all improve without losing the foundation.
