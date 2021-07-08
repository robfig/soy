package pomsg

import (
	"testing"
	"golang.org/x/text/language"
)

func TestFallback(t *testing.T) {
	tests := []struct{
		name string
		tag language.Tag
		expectedTags []language.Tag
	}{
		{
			name: "When given a generic locale code, no extra fallbacks are provided",
			tag: language.MustParse("en"),
			expectedTags: []language.Tag{language.English},
		},
		{
			name: "When given a regional locale code, generic fallback is provided",
			tag: language.MustParse("en_US"),
			expectedTags: []language.Tag{language.AmericanEnglish, language.English},
		},
		{
			name: "When given a locale code with script, generic fallback is provided",
			tag: language.MustParse("ar_Arab"),
			expectedTags: []language.Tag{
				language.MustParse("ar_Arab"),
				language.Arabic,
			},
		},
		{
			name: "When given a locale code with script and region, generic and script fallbacks are provided",
			tag: language.MustParse("ar_Arab_EG"),
			expectedTags: []language.Tag{
				language.MustParse("ar_Arab_EG"),
				language.MustParse("ar_Arab"),
				language.Arabic,
			},
		},
		{
			name: "When given an empty tag, no fallbacks are provided",
			tag: language.Tag{},
			expectedTags: []language.Tag{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fb := fallbacks(test.tag)

			for i, tag := range test.expectedTags {
				if fb[i] != tag {
					t.Errorf("Expected tag %+v, got tag %+v", tag, fb[i])
				}
			}
		})
	}
}
