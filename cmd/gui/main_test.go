package main

import "testing"

func TestFirstURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"no url", "分享目录: /tmp\n端口: 8080", ""},
		{"single", "直链:\nhttp://192.168.1.5:8080/a.txt\n", "http://192.168.1.5:8080/a.txt"},
		{"https", "https://example.com/f", "https://example.com/f"},
		{"first of many", "http://10.0.0.1:1\nhttp://10.0.0.2:2", "http://10.0.0.1:1"},
		{"ignore scheme-less", "192.168.1.5:8080/x\nhttp://ok", "http://ok"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := firstURL(c.in); got != c.want {
				t.Errorf("firstURL(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
