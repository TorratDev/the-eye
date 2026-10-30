package spawner

import (
	"reflect"
	"testing"

	"github.com/TorratDev/the-eye/internal/herdr"
)

func TestPlanPanes(t *testing.T) {
	wanted := []string{"investigator-a", "investigator-b", "investigator-c"}
	cases := []struct {
		name  string
		panes []herdr.Pane
		want  []step
	}{
		{ // fresh tab: one unlabelled pane is named, the rest are split off it
			name:  "fresh",
			panes: []herdr.Pane{{ID: "p1"}},
			want: []step{
				{anchor: "p1", name: "investigator-a"},
				{split: true, anchor: "p1", name: "investigator-b"},
				{split: true, anchor: "p1", name: "investigator-c"},
			},
		},
		{ // second run: everything is already named, nothing to do
			name:  "reuse",
			panes: []herdr.Pane{{ID: "p1", Label: "investigator-a"}, {ID: "p2", Label: "investigator-b"}, {ID: "p3", Label: "investigator-c"}},
			want:  nil,
		},
		{ // a wanted name missing from a full tab is split, unrelated labels stay
			name:  "missing",
			panes: []herdr.Pane{{ID: "p1", Label: "investigator-a"}, {ID: "p2", Label: "other"}, {ID: "p3", Label: "investigator-b"}},
			want:  []step{{split: true, anchor: "p1", name: "investigator-c"}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := planPanes(wanted, c.panes)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("planPanes = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestPlanPanesEmptyTab(t *testing.T) {
	if _, err := planPanes([]string{"x"}, nil); err == nil {
		t.Error("planPanes on an empty tab succeeded, want error")
	}
}
