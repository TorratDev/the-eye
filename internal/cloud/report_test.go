package cloud

import "testing"

// A worker comment in the contract format is parsed by the hub.
func TestParseComment(t *testing.T) {
	body := Marker + "\nrole: executor\nRan the tests, all pass.\n\nDONE: tests pass; verification: go test ok; branch: 22-dispatch"
	r, ok, err := ParseComment(body)
	if err != nil || !ok || r.Role != "executor" || r.Final.Kind != "DONE" {
		t.Fatalf("ParseComment = %+v, %v, %v; want an executor DONE report", r, ok, err)
	}
}

func TestOrdinaryCommentIsNotAReport(t *testing.T) {
	if _, ok, err := ParseComment("DONE: looks fine to me"); ok || err != nil {
		t.Errorf("unmarked comment: ok = %v, err = %v", ok, err)
	}
}

func TestMalformedReportRefused(t *testing.T) {
	if _, ok, err := ParseComment(Marker + "\nrole: executor\nall done, nice"); !ok || err == nil {
		t.Errorf("marked report with no final line: ok = %v, err = %v; want an error", ok, err)
	}
	if _, ok, err := ParseComment(Marker + "\nDONE: no role line"); !ok || err == nil {
		t.Errorf("marked report with no role line: ok = %v, err = %v; want an error", ok, err)
	}
}
