package security

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InvisibleUnicodeWarning", func() {
	It("flags invisible and private-use characters while ignoring emoji joiners", func() {
		tests := []struct {
			content []byte
			want    string
			empty   bool
		}{
			{content: []byte("def\u200bgreet():\n    pass\n"), want: "zero width space"},
			{content: []byte("def\ue000greet():\n    pass\n"), want: "private-use code point"},
			{content: []byte("name\ufe0f = 1\n"), want: "variation selector"},
			{content: []byte("def g\u200dreet():\n    pass\n"), want: "zero width joiner"},
			{content: []byte("x = \"\xf0\x9f\x91\xa9\xe2\x80\x8d\xf0\x9f\x92\xbb\"\n"), empty: true},
		}

		for _, test := range tests {
			warning := InvisibleUnicodeWarning("app.py", test.content)
			if test.empty {
				Expect(warning).To(BeEmpty())
				continue
			}
			Expect(warning).To(ContainSubstring("security warning"))
			Expect(strings.Contains(warning, test.want)).To(BeTrue())
		}
	})
})
