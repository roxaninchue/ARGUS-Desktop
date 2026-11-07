// Package brand is the app's name in one place (as BrandName.kt on Argus Android).
// User-facing Go strings read NAME / UPPER from here, so a future rename is one line.
// The frontend twin is frontend/dist/brandname.js. Things that must NOT follow a rename
// (deployed hosts, wire names, sync AADs) are listed in docs/RENAME-ARGUS.md.
package brand

const (
	// NAME is the product name in running text: "Argus".
	NAME = "Argus"
	// UPPER is the wordmark: "ARGUS".
	UPPER = "ARGUS"
	// MAKER is the company that makes the app.
	MAKER = "Blazeinchue"
	// BYLINE is the About line.
	BYLINE = NAME + " by " + MAKER
)

// TierLabels are the model tiers' names, as on Argus Android: Insight (fast), Oracle (balanced),
// Panoptes (deepest). Labels only: the ids the gateway reads (blaze_family) stay spark / flare / inferno.
var TierLabels = map[string]string{"spark": "Insight", "flare": "Oracle", "inferno": "Panoptes"}
