package hub

import "errors"

// Claim takes a sub-issue locally, then shows the claim on GitHub. It returns false if the
// sub-issue is already claimed. If the GitHub write fails, the local claim is undone.
func Claim(st *Store, repo string, epic, number int, owner string) (bool, error) {
	ok, err := st.Claim(number, epic, owner)
	if err != nil || !ok {
		return ok, err
	}
	if err := SetClaimLabel(repo, number, true); err != nil {
		return false, errors.Join(err, st.Release(number, owner))
	}
	return true, nil
}

// Release clears the local claim and removes the marker from GitHub.
func Release(st *Store, repo string, number int, owner string) error {
	if err := st.Release(number, owner); err != nil {
		return err
	}
	return SetClaimLabel(repo, number, false)
}
