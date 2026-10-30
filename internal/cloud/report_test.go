package cloud

import "testing"

// A worker comment in the contract format is parsed by the hub.
func TestParseComment(t *testing.T) {
	body := Marker + "\nRan the tests, all pass.\n\nDONE: tests pass; verification: go test ok; branch: 22-dispatch"
	r, ok, err := ParseComment(body, "executor")
	if err != nil || !ok || r.Final.Kind != "DONE" {
		t.Fatalf("ParseComment = %+v, %v, %v; want a DONE report", r, ok, err)
	}
}

func TestOrdinaryCommentIsNotAReport(t *testing.T) {
	if _, ok, err := ParseComment("DONE: looks fine to me", "executor"); ok || err != nil {
		t.Errorf("unmarked comment: ok = %v, err = %v", ok, err)
	}
}

func TestMalformedReportRefused(t *testing.T) {
	if _, ok, err := ParseComment(Marker+"\nall done, nice", "executor"); !ok || err == nil {
		t.Errorf("marked report with no final line: ok = %v, err = %v; want an error", ok, err)
	}
}
