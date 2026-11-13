# Image resources

Verified sources for the homepage identity imagery. All sample URLs below
returned HTTP 200 on 13 November 2026. Terms are not confirmed: store the
provider in `media_assets.license_metadata` and do not redistribute the images
as your own.

## Team logos

| Sport | Pattern | Verified sample |
|---|---|---|
| Football | `https://a.espncdn.com/i/teamlogos/soccer/500/{espnTeamId}.png` | `.../359.png` (Arsenal) |
| Basketball | `https://a.espncdn.com/i/teamlogos/nba/500/{abbr}.png` | `.../lal.png` (Lakers) |
| NFL | `https://a.espncdn.com/i/teamlogos/nfl/500/{abbr}.png` | `.../kc.png` (Chiefs) |
| National teams | `https://a.espncdn.com/i/teamlogos/countries/500/{iso3lower}.png` | `.../cze.png` (Czechia) |

Fallbacks with broader coverage:

- TheSportsDB: `strBadge` from `searchteams.php?t={name}` (key `123`), e.g.
  `https://r2.thesportsdb.com/images/media/team/badge/9kgami1655120871.png`.
- FotMob: `https://images.fotmob.com/image_resources/logo/teamlogo/{fotmobId}.png`
  (verified with id 9825). Not in the API proxy allowlist yet.

## Competition logos

- ESPN league feed: `leagues[0].logos[].href`, e.g. Premier League
  `https://a.espncdn.com/i/leaguelogos/soccer/500/23.png`.
- TheSportsDB: `lookupleague.php?id={id}` then `strBadge`:
  - NBA `4387`, NFL `4391`, ATP World Tour `4464`,
    Premier League `4328`.

## Player images

- ESPN tennis scoreboard exposes a `flag` per athlete but `headshot` is null on
  the free feed. Use the flag or initials, not a stock face.
  Pattern: `https://a.espncdn.com/i/teamlogos/countries/500/{iso3lower}.png`.
- No verified free source for player headshots across football, basketball or
  NFL. A licensed provider is required for real faces.

## Country flags

- `https://flagcdn.com/w80/{iso2lower}.png` (verified with `ng.png`).
- ESPN country badges (table above) are already used for national teams.

## Sport icons

Font Awesome free solid is installed and already used by the navigation:

- football: `faFutbol`
- tennis: `faTableTennisPaddleBall`
- basketball: `faBasketball`
- NFL: `faFootball`

Import them from `@fortawesome/free-solid-svg-icons`. They are vectors, so no
network request and no licence ambiguity.

## Hero imagery

There is no rights-cleared athlete photography in the project. Do not reuse the
concept image's athletes. Use the CSS pitch-lines and green glow already in the
theme, or a licensed decorative stock photo that is never used to identify a
team or player.

## Using them in the app

`media_assets` already allows `team`, `competition`, `player` entity types and
`team_logo`, `competition_logo`, `player_photo` asset types. The API proxy is
`GET /api/v1/teams/{id}/crest` with an allowlist: `a.espncdn.com`,
`r2.thesportsdb.com`, `www.thesportsdb.com`. Add the provider host before
storing a URL from it.

Fallback order for any identity:

1. verified provider image stored in `media_assets`
2. canonical stored image
3. initials in a neutral tile
4. sport icon

Never use an unrelated sports photograph as a team or player fallback.
