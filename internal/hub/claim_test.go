package hub

import (
	"errors"
	"path/filepath"
	"testing"
)

// fakeLabel replaces the GitHub marker writer for one test; fail makes the call with that on value fail.
func fakeLabel(t *testing.T, fail map[bool]bool) *[]bool {
	t.Helper()
	var calls []bool
	old := setClaimLabel
	setClaimLabel = func(repo string, n int, on bool) error {
		calls = append(calls, on)
		if fail[on] {
			return errors.New("gh down")
		}
		return nil
	}
	t.Cleanup(func() { setClaimLabel = old })
	return &calls
}

func TestClaimRollsBackWhenLabelFails(t *testing.T) {
	st := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	fakeLabel(t, map[bool]bool{true: true})
	ok, err := Claim(st, "o/r", 8, 4, "run-a")
	if ok || err == nil {
		t.Fatalf("claim = %v, %v; want false and an error", ok, err)
	}
	if claims, _ := st.Claims(); len(claims) != 0 {
		t.Fatalf("store claim kept after label failure: %v", claims)
	}
}

func TestReleaseKeepsClaimWhenLabelRemovalFails(t *testing.T) {
	st := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	calls := fakeLabel(t, map[bool]bool{false: true})
	if ok, err := Claim(st, "o/r", 8, 4, "run-a"); !ok || err != nil {
		t.Fatalf("claim = %v, %v", ok, err)
	}
	if err := Release(st, "o/r", 4, "run-a"); err == nil {
		t.Fatal("release succeeded although the label removal failed")
	}
	if claims, _ := st.Claims(); claims[4] != "run-a" {
		t.Fatalf("local claim dropped after label removal failed: %v", claims)
	}
	if len(*calls) != 2 || (*calls)[1] {
		t.Fatalf("label calls = %v, want [true false]", *calls)
	}
}

func TestReleaseRemovesLabelThenClaim(t *testing.T) {
	st := openTest(t, filepath.Join(t.TempDir(), "state.db"))
	calls := fakeLabel(t, nil)
	if ok, err := Claim(st, "o/r", 8, 4, "run-a"); !ok || err != nil {
		t.Fatalf("claim = %v, %v", ok, err)
	}
	if err := Release(st, "o/r", 4, "run-a"); err != nil {
		t.Fatal(err)
	}
	if claims, _ := st.Claims(); len(claims) != 0 {
		t.Fatalf("claims after release = %v, want none", claims)
	}
	if len(*calls) != 2 || !(*calls)[0] || (*calls)[1] {
		t.Fatalf("label calls = %v, want [true false]", *calls)
	}
}
