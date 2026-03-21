package indexer

import (
	"fmt"
	"path/filepath"
	"unicode"
	"unicode/utf8"
)

var invisibleRunes = map[rune]string{
	'\u00ad': "soft hyphen",
	'\u034f': "combining grapheme joiner",
	'\u061c': "arabic letter mark",
	'\u180e': "mongolian vowel separator",
	'\u200b': "zero width space",
	'\u200c': "zero width non-joiner",
	'\u200d': "zero width joiner",
	'\u200e': "left-to-right mark",
	'\u200f': "right-to-left mark",
	'\u202a': "left-to-right embedding",
	'\u202b': "right-to-left embedding",
	'\u202c': "pop directional formatting",
	'\u202d': "left-to-right override",
	'\u202e': "right-to-left override",
	'\u2060': "word joiner",
	'\u2066': "left-to-right isolate",
	'\u2067': "right-to-left isolate",
	'\u2068': "first strong isolate",
	'\u2069': "pop directional isolate",
	'\ufeff': "byte order mark",
}

// InvisibleUnicodeWarning returns a warning string when source content contains
// suspicious invisible Unicode characters that can obscure review.
func InvisibleUnicodeWarning(path string, content []byte) string {
	runes := make([]rune, 0, len(content))
	for len(content) > 0 {
		decoded, size := utf8.DecodeRune(content)
		if decoded == utf8.RuneError && size == 1 {
			content = content[size:]
			continue
		}
		runes = append(runes, decoded)
		content = content[size:]
	}

	for index, r := range runes {
		if (r == '\u200c' || r == '\u200d') && !isSuspiciousJoinerContext(runes, index) {
			continue
		}
		if name, ok := invisibleRunes[r]; ok {
			return fmt.Sprintf("security warning: %s contains invisible Unicode character %q (%s)", filepath.ToSlash(path), r, name)
		}
		if name, ok := invisibleRuneClass(r); ok {
			return fmt.Sprintf("security warning: %s contains invisible Unicode character %q (%s)", filepath.ToSlash(path), r, name)
		}
	}
	return ""
}

func invisibleRuneClass(r rune) (string, bool) {
	switch {
	case isVariationSelector(r):
		return "variation selector", true
	case isPrivateUseRune(r):
		return "private-use code point", true
	default:
		return "", false
	}
}

func isVariationSelector(r rune) bool {
	return (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0xE0100 && r <= 0xE01EF)
}

func isPrivateUseRune(r rune) bool {
	return unicode.Is(unicode.Co, r)
}

func isSuspiciousJoinerContext(runes []rune, index int) bool {
	if index <= 0 || index >= len(runes)-1 {
		return false
	}
	return isASCIIIdentifierRune(runes[index-1]) && isASCIIIdentifierRune(runes[index+1])
}

func isASCIIIdentifierRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}
