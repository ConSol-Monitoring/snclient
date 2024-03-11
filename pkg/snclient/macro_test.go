package snclient

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMacros(t *testing.T) {
	macros := map[string]string{
		"seconds":    "130",
		"unix time":  "1700834034",
		"float":      "170.05",
		"yesterday":  fmt.Sprintf("%d", time.Now().Add(-24*time.Hour).Unix()),
		"characters": `öäüß@€utf`,
	}

	tests := []struct {
		In     string
		Expect string
	}{
		{In: "$(seconds)", Expect: "130"},
		{In: "-${seconds}-", Expect: "-130-"},
		{In: "$(seconds:duration)", Expect: "2m 10s"},
		{In: "$(unix time:utc)", Expect: "2023-11-24 13:53:54 UTC"},
		{In: "$(unix time:utc:uc)", Expect: "2023-11-24 13:53:54 UTC"},
		{In: "$(unix time:utc:lc)", Expect: "2023-11-24 13:53:54 utc"},
		{In: "$(unix time : utc : lc)", Expect: "2023-11-24 13:53:54 utc"},
		{In: "$(unix time | utc | lc)", Expect: "2023-11-24 13:53:54 utc"},
		{In: "$(something else | utc | lc)", Expect: "$(something else | utc | lc)"},
		{In: "$(float | fmt=%d)", Expect: "170"},
		{In: "$(float | fmt=%.1f)", Expect: "170.1"},
		{In: "$(float | fmt=%.2f)", Expect: "170.05"},
		{In: "$(yesterday | age | duration)", Expect: "1d 00:00h"},
		{In: "$(characters | s/[^a-zA-Z]//g )", Expect: "utf"},
		{In: "$(characters | 's/^(.*?)([a-z]+)$/$2$1/g' )", Expect: "utföäüß@€"},
		{In: "${characters | 's/^(.*)ß.*?([a-z]+)/$2$1/g' }", Expect: "utföäü"},
		{In: "${characters | 's/.*(u{1}).*/U/g' }", Expect: "U"},
		{In: "${characters | 's/\\W//' }", Expect: "utf"},
		{In: "${characters | ascii }", Expect: "utf"},
		{In: "$(seconds)$(seconds)", Expect: "130130"},
		{In: "...'$(seconds)'...", Expect: "...'130'..."},
	}

	for i, tst := range tests {
		res := ReplaceMacros(tst.In, macros)
		assert.Equalf(t, tst.Expect, res, "[%d] replacing: %s", i, tst.In)
	}
}

func TestMacroToken(t *testing.T) {
	tests := []struct {
		In     string
		Expect []string
	}{
		{In: "...'$(seconds)'...", Expect: []string{"...'", "$(seconds)", "'..."}},
		{In: "... $(seconds | 's/a/b/' ) ...", Expect: []string{"... ", "$(seconds | 's/a/b/' )", " ..."}},
		{In: "... $(var | 's/\\)//' ) ...", Expect: []string{"... ", "$(var | 's/\\)//' )", " ..."}},
		{In: "{{ IF condition }}yes$(macro){{ ELSE }}$(other)no{{ END }}", Expect: []string{"{{ IF condition }}", "yes", "$(macro)", "{{ ELSE }}", "$(other)", "no", "{{ END }}"}},
	}

	splitBy := map[string]string{
		"$(": ")",
		"${": "}",
		"%(": ")",
		"%{": "}",
		"{{": "}}",
	}
	for i, tst := range tests {
		token, err := splitToken(tst.In, splitBy)
		require.NoErrorf(t, err, "[%d] text: %s", i, tst.In)
		assert.Equalf(t, tst.Expect, token, "replacing: %s", tst.In)
	}
}

func TestMacroConditionals(t *testing.T) {
	macros := map[string]string{
		"seconds":   "130",
		"unix time": "1700834034",
		"float":     "170.05",
		"yesterday": fmt.Sprintf("%d", time.Now().Add(-24*time.Hour).Unix()),
	}

	tests := []struct {
		In     string
		Expect string
	}{
		{In: "{{ IF seconds > 120 }}yes: $(seconds){{ ELSE }}no{{ END }}", Expect: "yes: 130"},
		{In: "{{ IF seconds > 180 }}yes: $(seconds){{ ELSIF seconds > 120 }}elsif: $(seconds){{ ELSE }}no{{ END }}", Expect: "elsif: 130"},
		{In: "{{ IF seconds > 120 }}outer$(seconds)-{{ IF seconds > 150 }}this not{{ ELSE }}inner$(seconds){{ END }}{{ ELSE }}also not{{ END }}", Expect: "outer130-inner130"},
	}

	for _, tst := range tests {
		res, err := ReplaceConditionals(tst.In, macros)
		require.NoError(t, err)
		res = ReplaceMacros(res, macros)
		assert.Equalf(t, tst.Expect, res, "replacing: %s", tst.In)
	}
}
