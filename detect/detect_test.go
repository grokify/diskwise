package detect

import "testing"

func TestUnderRoot(t *testing.T) {
	cases := []struct {
		path, root string
		want       bool
	}{
		{"/a/b", "/a/b", true},
		{"/a/b/c", "/a/b", true},
		{"/a/bc", "/a/b", false}, // shared string prefix, not a real descendant
		{"/a/c", "/a/b", false},
	}
	for _, c := range cases {
		if got := underRoot(c.path, c.root); got != c.want {
			t.Errorf("underRoot(%q, %q) = %v, want %v", c.path, c.root, got, c.want)
		}
	}
}
