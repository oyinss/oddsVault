// Package normalize turns provider entity names into stable match keys.
//
// Matching is deliberately conservative: an alias must be explicit, and a name
// that cannot be resolved keeps its own key rather than being force-matched to
// a similar team. Keys are for lookup only and are never shown to users.
package normalize

import (
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// clubTokens are stripped from either end of a name before comparison.
var clubTokens = map[string]struct{}{
	"fc": {}, "afc": {}, "cf": {}, "sc": {}, "ac": {}, "cfc": {}, "sfc": {},
	"club": {}, "calcio": {}, "futbol": {}, "football": {},
	"cd": {}, "ud": {}, "sv": {}, "vfb": {}, "vfl": {}, "tsg": {}, "fsv": {},
	"ssc": {}, "ss": {}, "as": {}, "us": {}, "rc": {}, "rcd": {}, "bc": {}, "acf": {},
	"osc": {}, "aj": {}, "ogc": {}, "sk": {}, "fk": {}, "bk": {}, "if": {},
}

// aliasPairs resolve provider spellings to canonical display names.
// Both sides are normalized through Key, so the left column may keep its
// natural spelling and punctuation.
var aliasPairs = [][2]string{
	// Premier League
	{"Man United", "Manchester United"},
	{"Man Utd", "Manchester United"},
	{"Manchester Utd", "Manchester United"},
	{"Man City", "Manchester City"},
	{"Newcastle", "Newcastle United"},
	{"Nott'm Forest", "Nottingham Forest"},
	{"Tottenham", "Tottenham Hotspur"},
	{"Spurs", "Tottenham Hotspur"},
	{"Wolves", "Wolverhampton Wanderers"},
	{"Brighton", "Brighton & Hove Albion"},
	{"Leeds", "Leeds United"},
	{"Ipswich", "Ipswich Town"},
	{"Hull", "Hull City"},
	{"Coventry", "Coventry City"},
	{"Bournemouth", "AFC Bournemouth"},
	{"West Ham", "West Ham United"},
	{"West Brom", "West Bromwich Albion"},
	{"Sheff Utd", "Sheffield United"},
	{"Sheffield Utd", "Sheffield United"},
	{"QPR", "Queens Park Rangers"},
	{"Leicester", "Leicester City"},
	{"Norwich", "Norwich City"},
	{"Stoke", "Stoke City"},
	{"Swansea", "Swansea City"},
	{"Derby", "Derby County"},
	{"Birmingham", "Birmingham City"},
	{"Cardiff", "Cardiff City"},
	{"Blackburn", "Blackburn Rovers"},
	{"Bolton", "Bolton Wanderers"},
	{"Preston", "Preston North End"},
	{"Charlton", "Charlton Athletic"},
	{"Lincoln", "Lincoln City"},
	{"Millwall", "Millwall"},
	{"Middlesbrough", "Middlesbrough"},

	// International
	{"Czech Republic", "Czechia"},

	// La Liga
	{"Ath Madrid", "Atlético Madrid"},
	{"Atletico Madrid", "Atlético Madrid"},
	{"Ath Bilbao", "Athletic Club"},
	{"Athletic Bilbao", "Athletic Club"},
	{"Sociedad", "Real Sociedad"},
	{"Betis", "Real Betis"},
	{"Vallecano", "Rayo Vallecano"},
	{"Espanol", "Espanyol"},
	{"Barcelona", "Barcelona"},
	{"Real Madrid", "Real Madrid"},
	{"Celta", "Celta Vigo"},
	{"Alaves", "Deportivo Alavés"},
	{"Dep. A Coruna", "Deportivo La Coruña"},
	{"La Coruna", "Deportivo La Coruña"},
	{"Malaga", "Málaga"},
	{"Santander", "Racing Santander"},
	{"Oviedo", "Real Oviedo"},
	{"Sp Gijon", "Sporting Gijón"},

	// Bundesliga
	{"Bayern Munich", "Bayern Munich"},
	{"FC Bayern Munchen", "Bayern Munich"},
	{"Dortmund", "Borussia Dortmund"},
	{"M'gladbach", "Borussia Mönchengladbach"},
	{"Borussia M.Gladbach", "Borussia Mönchengladbach"},
	{"Leverkusen", "Bayer Leverkusen"},
	{"Mainz", "Mainz 05"},
	{"Union Berlin", "Union Berlin"},
	{"FC Koln", "FC Köln"},
	{"FC Cologne", "FC Köln"},
	{"Hoffenheim", "Hoffenheim"},
	{"Werder Bremen", "Werder Bremen"},
	{"Ein Frankfurt", "Eintracht Frankfurt"},
	{"Hamburg", "Hamburger SV"},
	{"Stuttgart", "VfB Stuttgart"},

	// Serie A
	{"Milan", "AC Milan"},
	{"Inter", "Inter Milan"},
	{"Juventus", "Juventus"},
	{"Roma", "Roma"},
	{"Lazio", "Lazio"},
	{"Atalanta", "Atalanta"},
	{"Bologna", "Bologna"},
	{"Cagliari", "Cagliari"},
	{"Genoa", "Genoa"},
	{"Torino", "Torino"},
	{"Udinese", "Udinese"},
	{"Venezia", "Venezia"},
	{"Parma", "Parma"},
	{"Como", "Como"},
	{"Lecce", "Lecce"},
	{"Monza", "Monza"},
	{"Sassuolo", "Sassuolo"},
	{"Frosinone", "Frosinone"},

	// Ligue 1
	{"Paris SG", "Paris Saint-Germain"},
	{"Paris Saint Germain", "Paris Saint-Germain"},
	{"Marseille", "Marseille"},
	{"Olympique de Marseille", "Marseille"},
	{"Lyon", "Lyon"},
	{"Lens", "Lens"},
	{"Rennes", "Rennes"},
	{"Monaco", "Monaco"},
	{"Lille", "Lille"},
	{"Angers", "Angers SCO"},
	{"Auxerre", "AJ Auxerre"},
	{"Brest", "Stade Brestois 29"},
	{"Le Havre", "Le Havre AC"},
	{"Lorient", "FC Lorient"},
	{"Nice", "OGC Nice"},
	{"Strasbourg", "RC Strasbourg Alsace"},
	{"Toulouse", "Toulouse FC"},
	{"Troyes", "ES Troyes AC"},
}

var (
	aliasOnce sync.Once
	aliasMap  map[string]string
)

func aliases() map[string]string {
	aliasOnce.Do(func() {
		aliasMap = make(map[string]string, len(aliasPairs))
		for _, pair := range aliasPairs {
			k := Key(pair[0])
			canonical := strings.TrimSpace(pair[1])
			if existing, ok := aliasMap[k]; ok && existing != canonical {
				panic("normalize: conflicting alias " + k + ": " + existing + " vs " + canonical)
			}
			aliasMap[k] = canonical
		}
	})
	return aliasMap
}

// Key returns the comparison key for a provider or canonical name.
func Key(name string) string {
	folded := fold(name)
	tokens := strings.Fields(folded)
	tokens = trimClubTokens(tokens)
	return strings.Join(tokens, " ")
}

// Display returns a presentable name with trailing club tokens removed.
func Display(name string) string {
	cleaned := strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
	if cleaned == "" {
		return ""
	}
	words := strings.Fields(cleaned)
	for len(words) > 1 {
		last := strings.ToLower(stripPunctuation(words[len(words)-1]))
		if _, ok := clubTokens[last]; !ok {
			break
		}
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// Resolve maps a provider name to a canonical display name and match key.
func Resolve(name string) (display, key string) {
	k := Key(name)
	if canonical, ok := aliases()[k]; ok {
		return canonical, Key(canonical)
	}
	return Display(name), k
}

// Slug builds a URL-safe identifier from a key.
func Slug(key string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range key {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func fold(name string) string {
	decomposed := norm.NFKD.String(name)
	var b strings.Builder
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '&':
			b.WriteString(" and ")
		default:
			b.WriteByte(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func trimClubTokens(tokens []string) []string {
	for len(tokens) > 1 {
		if _, ok := clubTokens[tokens[0]]; ok {
			tokens = tokens[1:]
			continue
		}
		if isNumeric(tokens[0]) {
			tokens = tokens[1:]
			continue
		}
		break
	}
	for len(tokens) > 1 {
		if _, ok := clubTokens[tokens[len(tokens)-1]]; ok {
			tokens = tokens[:len(tokens)-1]
			continue
		}
		break
	}
	return tokens
}

func isNumeric(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func stripPunctuation(word string) string {
	var b strings.Builder
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
