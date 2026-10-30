package hub

import "errors"

// setClaimLabel writes the GitHub claim marker; tests replace it to run without gh.
var setClaimLabel = SetClaimLabel

// Claim takes a sub-issue locally, then shows the claim on GitHub. It returns false if the
// sub-issue is already claimed. If the GitHub write fails, the local claim is undone.
func Claim(st *Store, repo string, epic, number int, owner string) (bool, error) {
	ok, err := st.Claim(number, epic, owner)
	if err != nil || !ok {
		return ok, err
	}
	if err := setClaimLabel(repo, number, true); err != nil {
		return false, errors.Join(err, st.Release(number, owner))
	}
	return true, nil
}

// Release removes the marker from GitHub, then clears the local claim. If the marker
// cannot be removed, the local claim stays so the sub-issue is not handed out twice.
func Release(st *Store, repo string, number int, owner string) error {
	if err := setClaimLabel(repo, number, false); err != nil {
		return err
	}
	return st.Release(number, owner)
}
