package pomsg

import (
	"testing"
	"golang.org/x/text/language"
)

func TestFallback(t *testing.T) {
	tests := []struct{
		name string
		localeCode string
		expectedTags []language.Tag
	}{
		{
			name: "When given a generic locale code, no extra fallbacks are provided",
			localeCode: "en",
			expectedTags: []language.Tag{language.English},
		},
		{
			name: "When given a regional locale code, generic fallback is provided",
			localeCode: "en_US",
			expectedTags: []language.Tag{language.AmericanEnglish, language.English},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tag := language.MustParse(test.localeCode)
			fb := fallbacks(tag)

			for i, tag := range test.expectedTags {
				if fb[i] != tag {
					t.Errorf("Expected tag %+v, got tag %+v", tag, fb[i])
				}
			}
		})
	}
}
