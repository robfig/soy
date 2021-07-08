package pomsg

import (
	"golang.org/x/text/language"
)

// fallbacks returns a slice of tags that can be substituted for a tag, ordered by increasing
// generality.
// TODO: potentially support extensions and variants
func fallbacks(tag language.Tag) []language.Tag {
	result := []language.Tag{}
	lang, script, region := tag.Raw()
	// The language package returns ZZ for an unspecified region, similar quirk for script.
	if region.String() != "ZZ" {
		t, _ := language.Compose(lang, script, region)
		result = append(result, t)
	}
	if script.String() != "Zzzz" {
		t, _ := language.Compose(lang, script)
		result = append(result, t)
	}
	t, _ := language.Compose(lang)
	result = append(result, t)
	return result
}
