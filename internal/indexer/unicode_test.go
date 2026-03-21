package indexer

import "testing"

func TestInvisibleUnicodeWarningWrapper(t *testing.T) {
	if InvisibleUnicodeWarning("app.py", []byte("def\u200bgreet():\n    pass\n")) == "" {
		t.Fatal("expected top-level unicode wrapper to delegate")
	}
}
