package indexer

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InvisibleUnicodeWarning", func() {
	It("flags classic zero-width characters", func() {
		warning := InvisibleUnicodeWarning("app.py", []byte("def\u200bgreet():\n    pass\n"))
		Expect(warning).To(ContainSubstring("security warning"))
		Expect(warning).To(ContainSubstring("zero width space"))
	})

	It("flags private-use code points", func() {
		warning := InvisibleUnicodeWarning("app.py", []byte("def\ue000greet():\n    pass\n"))
		Expect(warning).To(ContainSubstring("security warning"))
		Expect(warning).To(ContainSubstring("private-use code point"))
	})

	It("flags variation selectors", func() {
		warning := InvisibleUnicodeWarning("app.py", []byte("name\ufe0f = 1\n"))
		Expect(warning).To(ContainSubstring("security warning"))
		Expect(warning).To(ContainSubstring("variation selector"))
	})

	It("flags joiners when embedded in ASCII identifiers", func() {
		warning := InvisibleUnicodeWarning("app.py", []byte("def g\u200dreet():\n    pass\n"))
		Expect(warning).To(ContainSubstring("security warning"))
		Expect(warning).To(ContainSubstring("zero width joiner"))
	})

	It("ignores joiners in common emoji sequences", func() {
		warning := InvisibleUnicodeWarning("app.py", []byte("x = \"\xf0\x9f\x91\xa9\xe2\x80\x8d\xf0\x9f\x92\xbb\"\n"))
		Expect(warning).To(BeEmpty())
	})
})
