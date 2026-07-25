package model

// MoodPresets is the canonical list of mood tags shown across the app — the
// garden's mood-sticker collection ("collected" vs "uncollected") diffs
// against this same list. daily_logs.mood itself stays freeform text (not
// constrained to this list at the DB layer), so a user's collected moods
// can in principle include values outside this set too.
var MoodPresets = []string{
	"senang", "tenang", "biasa", "sensitif", "cemas", "sedih", "mudah marah",
}
