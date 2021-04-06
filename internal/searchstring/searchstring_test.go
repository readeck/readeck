package searchstring

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var searchTermsTests = []struct {
	s      string
	expect []SearchTerm
}{
	{"\u00A0 simple \t\u200a\n test \u1680", []SearchTerm{
		{Quotes: false, Field: "", Value: "simple"},
		{Quotes: false, Field: "", Value: "test"},
	}},
	{`"a test" to multiple "unclosed`, []SearchTerm{
		{Quotes: true, Field: "", Value: `a test`},
		{Quotes: false, Field: "", Value: "to"},
		{Quotes: false, Field: "", Value: "multiple"},
		{Quotes: true, Field: "", Value: `unclosed`},
	}},
	{`"quoted" "q \"test" "tt\ab"`, []SearchTerm{
		{Quotes: true, Field: "", Value: "quoted"},
		{Quotes: true, Field: "", Value: `q "test`},
		{Quotes: true, Field: "", Value: `tt\ab`},
	}},
	{`title:test other:"long string" bar:foo string`, []SearchTerm{
		{Quotes: false, Field: "title", Value: "test"},
		{Quotes: true, Field: "other", Value: `long string`},
		{Quotes: false, Field: "bar", Value: "foo"},
		{Quotes: false, Field: "", Value: "string"},
	}},
	{"", []SearchTerm{}},
	{`"`, []SearchTerm{
		{Quotes: true, Value: ""},
	}},
	{"🦊 title:🐼", []SearchTerm{
		{Field: "", Value: "🦊"},
		{Field: "title", Value: "🐼"},
	}},
}

func TestSearchTerms(t *testing.T) {
	for _, tt := range searchTermsTests {
		actual, err := Parse(tt.s)
		assert.Equal(t, nil, err)
		assert.Equal(t, tt.expect, actual)
	}
}

func TestSearchError(t *testing.T) {
	res, err := Parse("field1:field2:test")
	assert.Equal(t, []SearchTerm(nil), res)
	assert.EqualError(t, err, "field followed by a field")
}
