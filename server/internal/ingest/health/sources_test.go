package health

import "testing"

func TestCanonicalSource(t *testing.T) {
	cases := []struct {
		name, bundle, display, want string
	}{
		{"older client keeps what it sent", "", "Linus Watch Ultra 2", "Linus Watch Ultra 2"},
		{"older client without a name stays Apple Health", "", "", ""},
		{"a watch is Apple Health", "com.apple.health.5C1F0A77-3E0B-4B8C-9F0D-1D2A3B4C5D6E", "Linus Watch Ultra 2", ""},
		{"an Apple app is Apple Health", "com.apple.Health", "Health", ""},
		{"Oura by bundle", "com.ouraring.oura", "Oura", "Oura"},
		{"Withings by bundle", "com.withings.wiScaleNG", "Withings", "Withings"},
		{"a directly synced provider under an unlisted bundle", "com.ouraring.oura.beta", "oura", "Oura"},
		{"another app keeps its name", "com.strongapp.strong", "Strong", "Strong"},
		{"another app without a name keeps its bundle", "com.example.tracker", "", "com.example.tracker"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CanonicalSource(c.bundle, c.display); got != c.want {
				t.Errorf("CanonicalSource(%q, %q) = %q, want %q", c.bundle, c.display, got, c.want)
			}
		})
	}
}

func TestSourcePolicyDropsOnlyCopiesWithABundle(t *testing.T) {
	policy := sourcePolicy{direct: map[string]bool{"Oura": true}}

	if _, drop := policy.resolve("com.ouraring.oura", "Oura"); !drop {
		t.Error("an Oura HealthKit copy is kept although Oura syncs directly")
	}
	// An older app sends the display name without a bundle; its items keep the
	// treatment they had before the policy existed.
	if _, drop := policy.resolve("", "Oura"); drop {
		t.Error("an item without a bundle is dropped")
	}
	if _, drop := policy.resolve("com.withings.wiScaleNG", "Withings"); drop {
		t.Error("a Withings copy is dropped although Withings is not synced directly")
	}
}
