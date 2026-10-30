package main

import "testing"

func TestParseRepo(t *testing.T) {
	for _, in := range []string{
		"TorratDev/the-eye",
		"https://github.com/TorratDev/the-eye.git",
		"https://github.com/TorratDev/the-eye/",
		"git@github.com:TorratDev/the-eye.git",
	} {
		owner, name, err := parseRepo(in)
		if err != nil || owner != "TorratDev" || name != "the-eye" {
			t.Errorf("parseRepo(%q) = %q, %q, %v", in, owner, name, err)
		}
	}
	if _, _, err := parseRepo("the-eye"); err == nil {
		t.Error("parseRepo(\"the-eye\") succeeded, want error")
	}
}
