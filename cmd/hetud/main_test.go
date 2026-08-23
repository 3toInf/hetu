package main

import "testing"

func TestServeFlags(t *testing.T) {
	root := newServeCmd()
	_ = root.ParseFlags([]string{"--web-addr=127.0.0.1:0", "--no-web"})
	addr, _ := root.Flags().GetString("web-addr")
	noweb, _ := root.Flags().GetBool("no-web")
	if addr != "127.0.0.1:0" || !noweb {
		t.Fatalf("flags not parsed: %s %v", addr, noweb)
	}
}
