package claude

import "testing"

func TestDecodeProjectSlug(t *testing.T) {
	cases := map[string]string{
		"-home-u-code-alpha": "/home/u/code/alpha",
		"-Users-u-code-alpha": "/Users/u/code/alpha",
	}
	for slug, want := range cases {
		got := DecodeProjectSlug(slug)
		if got != want {
			t.Errorf("DecodeProjectSlug(%q)=%q want %q", slug, got, want)
		}
	}
}
