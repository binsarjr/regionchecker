package rdap

import (
	"encoding/json"
	"regexp"
	"strings"
)

// privacyProxyRE matches organisations that sit as privacy proxies on
// behalf of the true registrant. Their "country" is their own corporate
// address (typically US), not the domain owner's. When the registrant
// or registrar org matches, the RDAP result is treated as redacted.
var privacyProxyRE = regexp.MustCompile(`(?i)\b(?:` +
	`cloudflare|` +
	`domains\s+by\s+proxy|` +
	`whoisguard|` +
	`privacy\s*protect|` +
	`perfect\s+privacy|` +
	`privacyguardian|` +
	`contact\s+privacy|` +
	`withheld\s+for\s+privacy|` +
	`registration\s+private|` +
	`redacted\s+for\s+privacy|` +
	`proxy\s+protection|` +
	`identity\s+protection` +
	`)\b`)

// IsPrivacyProxy reports whether org matches a known privacy-proxy pattern.
func IsPrivacyProxy(org string) bool {
	if org == "" {
		return false
	}
	return privacyProxyRE.MatchString(org)
}

// registrantInfo is what we extract from an RDAP response.
type registrantInfo struct {
	Country string
	// OrgName first-seen, for debugging.
	Org string
}

// extractRegistrant walks the RDAP domain response and returns the
// registrant entity's country code (ISO 3166 alpha-2, uppercased) when
// present. RDAP entities expose contact data as a jCard vcardArray —
// the "adr" property's parameters carry a `cc` value, and its text
// value's last element holds the country name. Many registries redact
// this field (GDPR, ICANN privacy) — callers must tolerate ("", false).
func extractRegistrant(raw []byte) (registrantInfo, bool) {
	var doc struct {
		Entities []rdapEntity `json:"entities"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return registrantInfo{}, false
	}
	info, ok := walkEntities(doc.Entities)
	if !ok {
		return registrantInfo{}, false
	}
	info.Country = strings.ToUpper(strings.TrimSpace(info.Country))
	if info.Country == "" {
		return info, false
	}
	return info, true
}

// rdapEntity is a subset of the RDAP entity schema — just the fields
// we walk to reach registrant data.
type rdapEntity struct {
	Roles      []string        `json:"roles"`
	VCardArray json.RawMessage `json:"vcardArray"`
	Entities   []rdapEntity    `json:"entities"`
}

// rdapLink captures the fields needed to find a registrar RDAP follow-up URL.
type rdapLink struct {
	Href string `json:"href"`
	Rel  string `json:"rel"`
	Type string `json:"type"`
}

// extractRelatedRDAP returns the href of the first "related" link with
// RDAP media type. Registry-level RDAP for gTLDs (e.g. Verisign .com)
// exposes only the registrar entity; full registrant data is often at
// the registrar's RDAP service, reachable through this link.
func extractRelatedRDAP(raw []byte) string {
	var doc struct {
		Links []rdapLink `json:"links"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	for _, l := range doc.Links {
		if strings.EqualFold(l.Rel, "related") && strings.Contains(strings.ToLower(l.Type), "rdap") {
			return l.Href
		}
	}
	return ""
}

func walkEntities(ents []rdapEntity) (registrantInfo, bool) {
	// Prefer explicit registrant entities; fall back to any entity with a
	// country parseable from its vcard.
	for _, e := range ents {
		if hasRole(e.Roles, "registrant") {
			if info, ok := vcardCountry(e.VCardArray); ok {
				if IsPrivacyProxy(info.Org) {
					// Privacy-proxied registrant: their country is the
					// proxy's, not the domain owner's. Drop.
					return registrantInfo{}, false
				}
				return info, true
			}
		}
		if info, ok := walkEntities(e.Entities); ok {
			return info, true
		}
	}
	// Second pass: any entity (registrar, admin). Skip privacy proxies.
	for _, e := range ents {
		if info, ok := vcardCountry(e.VCardArray); ok {
			if IsPrivacyProxy(info.Org) {
				continue
			}
			// Only accept non-registrant entities when the role is
			// clearly identity-bearing (admin, owner, technical).
			if !hasRole(e.Roles, "registrar") {
				return info, true
			}
		}
	}
	return registrantInfo{}, false
}

func hasRole(roles []string, want string) bool {
	for _, r := range roles {
		if strings.EqualFold(r, want) {
			return true
		}
	}
	return false
}

// vcardCountry parses a jCard ["vcard", [ [name, params, type, value], ... ]]
// and returns the country from the first "adr" property. Looks at both
// the `cc` parameter (ISO alpha-2) and the last element of the adr
// text value (country name — rare, we only match alpha-2 in params).
func vcardCountry(raw json.RawMessage) (registrantInfo, bool) {
	var top []json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil || len(top) < 2 {
		return registrantInfo{}, false
	}
	var props []json.RawMessage
	if err := json.Unmarshal(top[1], &props); err != nil {
		return registrantInfo{}, false
	}
	var info registrantInfo
	var ok bool
	for _, p := range props {
		var prop []json.RawMessage
		if err := json.Unmarshal(p, &prop); err != nil || len(prop) < 4 {
			continue
		}
		var name string
		if err := json.Unmarshal(prop[0], &name); err != nil {
			continue
		}
		switch strings.ToLower(name) {
		case "fn", "org":
			if info.Org == "" {
				_ = json.Unmarshal(prop[3], &info.Org)
			}
		case "adr":
			var params map[string]any
			_ = json.Unmarshal(prop[1], &params)
			if cc, found := params["cc"]; found {
				if s, sok := cc.(string); sok && len(s) == 2 {
					info.Country = s
					ok = true
				}
			}
			// Fallback: adr value array last element (country name).
			if !ok {
				var adrVal []any
				if err := json.Unmarshal(prop[3], &adrVal); err == nil && len(adrVal) == 7 {
					if s, sok := adrVal[6].(string); sok && len(s) == 2 {
						info.Country = s
						ok = true
					}
				}
			}
		}
	}
	return info, ok
}
