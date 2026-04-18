package domain

import "golang.org/x/net/publicsuffix"

// EffectiveTLD returns the public suffix and whether it is managed by ICANN.
func EffectiveTLD(host string) (etld string, icann bool) {
	return publicsuffix.PublicSuffix(host)
}

// RegisteredDomain returns the eTLD+1 for host.
func RegisteredDomain(host string) (string, error) {
	return publicsuffix.EffectiveTLDPlusOne(host)
}
