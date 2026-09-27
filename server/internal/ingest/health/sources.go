package health

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// appleBundlePrefix covers every Apple device and Apple app writing into
// HealthKit. A Watch or iPhone reports itself as com.apple.health.<UUID>, the
// Health, Fitness and Sleep apps under their own com.apple identifiers.
const appleBundlePrefix = "com.apple."

// knownBundles maps the HealthKit bundle identifiers of providers FreeReps
// also syncs directly to the source name their direct integration writes.
// Sharing the name is what lets the source priority and dropCopy treat both
// deliveries as one provider.
var knownBundles = map[string]string{
	"com.ouraring.oura":      "Oura",
	"com.withings.wiScaleNG": "Withings",
}

// CanonicalSource returns the source name a row from HealthKit is stored under.
//
//   - Without a bundle the client predates source_bundle; name is kept as it
//     was stored before, so rows from Health Auto Export and older app versions
//     do not change meaning.
//   - Every Apple device and app maps to "", the name the source priority knows
//     as Apple Health. Device names such as "Linus Watch Ultra 2" never match a
//     priority entry, and "" is what the stored Apple Health rows carry.
//   - A provider FreeReps syncs directly maps to that provider's name, by
//     bundle or, for a bundle not in knownBundles, by its display name.
//   - Any other app keeps its display name, and its bundle when it has none.
func CanonicalSource(bundle, name string) string {
	if bundle == "" {
		return name
	}
	if strings.HasPrefix(bundle, appleBundlePrefix) {
		return ""
	}
	if provider, ok := knownBundles[bundle]; ok {
		return provider
	}
	for _, provider := range knownBundles {
		if strings.EqualFold(name, provider) {
			return provider
		}
	}
	if name != "" {
		return name
	}
	return bundle
}

// sourcePolicy decides, per payload, under which name an item is stored and
// whether it is a HealthKit copy of a provider FreeReps syncs directly.
type sourcePolicy struct {
	direct map[string]bool
	// dropped counts the dropped items per bundle and display name. It is
	// logged, because a dropped item leaves no row whose source_bundle could
	// be read afterwards, and the log is where a provider's bundle identifier
	// is confirmed against knownBundles.
	dropped map[string]int
}

func (p *Provider) loadSourcePolicy(ctx context.Context, userID int) (*sourcePolicy, error) {
	providers, err := p.db.DirectSyncProviders(ctx, userID)
	if err != nil {
		return nil, err
	}
	direct := make(map[string]bool, len(providers))
	for _, name := range providers {
		direct[name] = true
	}
	return &sourcePolicy{direct: direct, dropped: map[string]int{}}, nil
}

// resolve returns the stored source name and whether the item is dropped.
//
// A copy is dropped when the provider that wrote it into HealthKit is also
// synced directly for this user. The two deliveries describe the same
// measurement under the same source name, and the client rank
// (storage.clientRankSQL) would put the HealthKit copy from the iOS app ahead
// of the provider's own row. This is the sleep rule of 2026-09-20
// (sleepClaimedBySync) applied to every data type; see DECISIONS.md,
// 2026-09-27.
//
// Only items carrying a bundle are dropped. An older app version reports none,
// and its items keep the treatment they had before.
func (s *sourcePolicy) resolve(bundle, name string) (source string, drop bool) {
	source = CanonicalSource(bundle, name)
	drop = bundle != "" && s.direct[source]
	if drop {
		s.dropped[fmt.Sprintf("%s (%s)", bundle, name)]++
	}
	return source, drop
}

// droppedSummary lists the dropped items as "bundle (name)=count", sorted so
// that repeated syncs log the same line for the same sources.
func (s *sourcePolicy) droppedSummary() string {
	keys := make([]string, 0, len(s.dropped))
	for k := range s.dropped {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%d", k, s.dropped[k])
	}
	return strings.Join(parts, ", ")
}
