package contentscan

import "regexp"

func init() {
	RegisterDetector(newID())
	RegisterDetector(newSG())
	RegisterDetector(newMY())
	RegisterDetector(newGB())
	RegisterDetector(newJP())
	RegisterDetector(newUS())
}

// ---------------- Indonesia (ID) ----------------

type idDetector struct {
	lang    *regexp.Regexp
	phone   *regexp.Regexp
	tld     *regexp.Regexp
	cities  []string
	legal   *regexp.Regexp
	money   *regexp.Regexp
	country *regexp.Regexp
	bahasa  *regexp.Regexp
}

func newID() *idDetector {
	return &idDetector{
		lang:    mustCompile(`(?i)\blang\s*=\s*["']?id(?:-ID)?\b`),
		phone:   mustCompile(`\+62[\s\-]?8\d`),
		tld:     mustCompile(`\b[a-z0-9-]+\.(?:co|go|or|ac|net|web|sch|mil)\.id\b`),
		cities:  []string{"Jakarta", "Surabaya", "Bandung", "Yogyakarta", "Medan", "Semarang", "Denpasar", "Makassar", "Palembang", "Bekasi", "Tangerang"},
		legal:   mustCompile(`\bPT\.?\s+[A-Z][A-Za-z&]`),
		money:   mustCompile(`(?i)\b(?:Rp\.?\s?\d|IDR\b)`),
		country: mustCompile(`(?i)\bIndonesia\b`),
		bahasa:  mustCompile(`(?i)\bbahasa\s+indonesia\b`),
	}
}

func (d *idDetector) CC() string { return "ID" }

func (d *idDetector) Score(b string) int {
	s := 0
	if d.lang.MatchString(b) {
		s += 3
	}
	s += 3 * countMatches(d.phone, b, 1)
	s += 2 * countMatches(d.tld, b, 2)
	s += countAnyFold(b, d.cities, 3)
	if d.legal.MatchString(b) {
		s += 2
	}
	if d.money.MatchString(b) {
		s += 2
	}
	if d.bahasa.MatchString(b) {
		s += 2
	}
	if d.country.MatchString(b) {
		s++
	}
	return s
}

// ---------------- Singapore (SG) ----------------

type sgDetector struct {
	lang    *regexp.Regexp
	phone   *regexp.Regexp
	tld     *regexp.Regexp
	cities  []string
	legal   *regexp.Regexp
	money   *regexp.Regexp
	country *regexp.Regexp
}

func newSG() *sgDetector {
	return &sgDetector{
		lang:    mustCompile(`(?i)\blang\s*=\s*["']?en-SG\b`),
		phone:   mustCompile(`\+65[\s\-]?[6-9]\d{3}`),
		tld:     mustCompile(`\b[a-z0-9-]+\.(?:com|edu|org|net)\.sg\b`),
		cities:  []string{"Singapore", "Orchard Road", "Marina Bay", "Changi", "Jurong", "Bishan"},
		legal:   mustCompile(`(?i)\bPte\.?\s+Ltd\.?\b`),
		money:   mustCompile(`(?i)\b(?:SGD\b|S\$\s?\d)`),
		country: mustCompile(`(?i)\bSingapore\b`),
	}
}

func (d *sgDetector) CC() string { return "SG" }

func (d *sgDetector) Score(b string) int {
	s := 0
	if d.lang.MatchString(b) {
		s += 3
	}
	s += 3 * countMatches(d.phone, b, 1)
	s += 2 * countMatches(d.tld, b, 2)
	s += countAnyFold(b, d.cities, 3)
	if d.legal.MatchString(b) {
		s += 2
	}
	if d.money.MatchString(b) {
		s += 2
	}
	if d.country.MatchString(b) {
		s++
	}
	return s
}

// ---------------- Malaysia (MY) ----------------

type myDetector struct {
	lang    *regexp.Regexp
	phone   *regexp.Regexp
	tld     *regexp.Regexp
	cities  []string
	legal   *regexp.Regexp
	money   *regexp.Regexp
	country *regexp.Regexp
}

func newMY() *myDetector {
	return &myDetector{
		lang:    mustCompile(`(?i)\blang\s*=\s*["']?(?:ms|en-MY)\b`),
		phone:   mustCompile(`\+60[\s\-]?1\d`),
		tld:     mustCompile(`\b[a-z0-9-]+\.(?:com|edu|org|net|gov)\.my\b`),
		cities:  []string{"Kuala Lumpur", "Penang", "Johor", "Selangor", "Sabah", "Sarawak", "Putrajaya", "Melaka"},
		legal:   mustCompile(`(?i)\bSdn\.?\s+Bhd\.?\b`),
		money:   mustCompile(`(?i)\b(?:MYR\b|RM\s?\d)`),
		country: mustCompile(`(?i)\bMalaysia\b`),
	}
}

func (d *myDetector) CC() string { return "MY" }

func (d *myDetector) Score(b string) int {
	s := 0
	if d.lang.MatchString(b) {
		s += 3
	}
	s += 3 * countMatches(d.phone, b, 1)
	s += 2 * countMatches(d.tld, b, 2)
	s += countAnyFold(b, d.cities, 3)
	if d.legal.MatchString(b) {
		s += 2
	}
	if d.money.MatchString(b) {
		s += 2
	}
	if d.country.MatchString(b) {
		s++
	}
	return s
}

// ---------------- United Kingdom (GB) ----------------

type gbDetector struct {
	lang    *regexp.Regexp
	phone   *regexp.Regexp
	tld     *regexp.Regexp
	cities  []string
	legal   *regexp.Regexp
	money   *regexp.Regexp
	country *regexp.Regexp
}

func newGB() *gbDetector {
	return &gbDetector{
		lang:    mustCompile(`(?i)\blang\s*=\s*["']?en-GB\b`),
		phone:   mustCompile(`\+44[\s\-]?[12378]\d`),
		tld:     mustCompile(`\b[a-z0-9-]+\.(?:co|ac|gov|org|me|ltd|plc)\.uk\b`),
		cities:  []string{"London", "Manchester", "Birmingham", "Edinburgh", "Glasgow", "Liverpool", "Bristol", "Leeds", "Cardiff", "Belfast"},
		legal:   mustCompile(`(?i)\b(?:Ltd\.|Limited|PLC)\b`),
		money:   mustCompile(`(?i)\b(?:GBP\b|£\s?\d)`),
		country: mustCompile(`(?i)\b(?:United\s+Kingdom|Great\s+Britain)\b`),
	}
}

func (d *gbDetector) CC() string { return "GB" }

func (d *gbDetector) Score(b string) int {
	s := 0
	if d.lang.MatchString(b) {
		s += 3
	}
	s += 3 * countMatches(d.phone, b, 1)
	s += 2 * countMatches(d.tld, b, 2)
	s += countAnyFold(b, d.cities, 3)
	// Legal suffix (Ltd/PLC) is global English → weaker weight.
	if d.legal.MatchString(b) {
		s++
	}
	if d.money.MatchString(b) {
		s += 2
	}
	if d.country.MatchString(b) {
		s++
	}
	return s
}

// ---------------- Japan (JP) ----------------

type jpDetector struct {
	lang    *regexp.Regexp
	phone   *regexp.Regexp
	tld     *regexp.Regexp
	cities  []string
	legal   *regexp.Regexp // 株式会社
	money   *regexp.Regexp
	country *regexp.Regexp
}

func newJP() *jpDetector {
	return &jpDetector{
		lang:    mustCompile(`(?i)\blang\s*=\s*["']?ja(?:-JP)?\b`),
		phone:   mustCompile(`\+81[\s\-]?[3-9]\d`),
		tld:     mustCompile(`\b[a-z0-9-]+\.(?:co|ne|or|ac|go|ad|ed)\.jp\b`),
		cities:  []string{"Tokyo", "Osaka", "Kyoto", "Yokohama", "Nagoya", "Sapporo", "Fukuoka"},
		legal:   mustCompile(`株式会社|有限会社`),
		money:   mustCompile(`(?i)\b(?:JPY\b|¥\s?\d)`),
		country: mustCompile(`(?i)\bJapan\b`),
	}
}

func (d *jpDetector) CC() string { return "JP" }

func (d *jpDetector) Score(b string) int {
	s := 0
	if d.lang.MatchString(b) {
		s += 3
	}
	s += 3 * countMatches(d.phone, b, 1)
	s += 2 * countMatches(d.tld, b, 2)
	s += countAnyFold(b, d.cities, 3)
	if d.legal.MatchString(b) {
		s += 3 // Kanji legal form is strong
	}
	if d.money.MatchString(b) {
		s += 2
	}
	if d.country.MatchString(b) {
		s++
	}
	return s
}

// ---------------- United States (US) ----------------

// US detection is weak from content alone: English is global, $ is
// also CAD/AUD/etc., +1 shared with CA. We only score obvious markers.
type usDetector struct {
	lang    *regexp.Regexp
	phone   *regexp.Regexp
	tld     *regexp.Regexp
	cities  []string
	money   *regexp.Regexp
	country *regexp.Regexp
	states  []string
}

func newUS() *usDetector {
	return &usDetector{
		lang:   mustCompile(`(?i)\blang\s*=\s*["']?en-US\b`),
		phone:  mustCompile(`\+1[\s\-]?\(?[2-9]\d{2}\)?[\s\-]?\d{3}[\s\-]?\d{4}`),
		tld:    mustCompile(`\.(?:gov|mil)\b`),
		cities: []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix", "San Francisco", "Seattle", "Boston", "Washington, DC"},
		money:  mustCompile(`(?i)\bUSD\b`),
		country: mustCompile(`(?i)\bUnited\s+States\s+of\s+America\b`),
		states: []string{", CA ", ", NY ", ", TX ", ", FL ", ", WA ", ", MA "},
	}
}

func (d *usDetector) CC() string { return "US" }

func (d *usDetector) Score(b string) int {
	s := 0
	if d.lang.MatchString(b) {
		s += 2 // weaker than ccTLD langs; en-US is default for many
	}
	s += 2 * countMatches(d.phone, b, 1)
	s += 3 * countMatches(d.tld, b, 1) // .gov/.mil is strong
	s += countAnyFold(b, d.cities, 2)
	s += countAnyFold(b, d.states, 2)
	if d.money.MatchString(b) {
		s++
	}
	if d.country.MatchString(b) {
		s++
	}
	return s
}
