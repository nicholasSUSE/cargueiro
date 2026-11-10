package index

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// fleet returns the given versions of the fleet chart.
func fleet(versions ...string) []Ref {
	refs := make([]Ref, 0, len(versions))
	for _, v := range versions {
		refs = append(refs, Ref{Chart: "fleet", Version: v})
	}
	return refs
}

func TestPlan(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  Plan
	}{
		{
			name:  "nothing at all",
			state: State{},
			want:  Plan{},
		},
		{
			name: "every version is already published",
			state: State{
				Entries:   fleet("1", "2"),
				Published: fleet("1", "2"),
				Archives:  fleet("1", "2"),
			},
			want: Plan{},
		},
		{
			name: "an entry that is not published yet",
			state: State{
				Entries:   fleet("1", "2"),
				Published: fleet("1"),
				Archives:  fleet("1"),
			},
			want: Plan{Add: fleet("2")},
		},
		{
			name: "everything is new on the first run",
			state: State{
				Entries: fleet("2", "1"),
			},
			want: Plan{Add: fleet("1", "2")},
		},
		{
			name: "new versions come sorted by chart and then by version",
			state: State{
				Entries: []Ref{
					{Chart: "fleet", Version: "2"},
					{Chart: "alerting", Version: "9"},
					{Chart: "fleet", Version: "1"},
				},
			},
			want: Plan{Add: []Ref{
				{Chart: "alerting", Version: "9"},
				{Chart: "fleet", Version: "1"},
				{Chart: "fleet", Version: "2"},
			}},
		},
		{
			name: "the same version listed twice is one version",
			state: State{
				Entries: fleet("1", "1"),
			},
			want: Plan{Add: fleet("1")},
		},
		{
			name: "a published version that is hidden is taken out of the index",
			state: State{
				Entries:   fleet("1", "2"),
				Hidden:    fleet("2"),
				Published: fleet("1", "2"),
				Archives:  fleet("1", "2"),
			},
			want: Plan{Hide: fleet("2")},
		},
		{
			name: "a version that is hidden already is left alone",
			state: State{
				Entries:   fleet("1", "2"),
				Hidden:    fleet("2"),
				Published: fleet("1"),
				Archives:  fleet("1", "2"),
			},
			want: Plan{},
		},
		{
			name: "versions to add and to hide at once",
			state: State{
				Entries:   fleet("1", "2", "3"),
				Hidden:    fleet("1"),
				Published: fleet("1", "2"),
				Archives:  fleet("1", "2"),
			},
			want: Plan{
				Add:  fleet("3"),
				Hide: fleet("1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.state.Plan()

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPlanFails(t *testing.T) {
	tests := []struct {
		name    string
		state   State
		wantErr []string
	}{
		{
			name: "a published version lost its entry",
			state: State{
				Entries:   fleet("1"),
				Published: fleet("1", "2"),
				Archives:  fleet("1", "2"),
			},
			wantErr: []string{"fleet 2 is in index.yaml but has no entry: entries are never removed"},
		},
		{
			name: "an archive with no entry",
			state: State{
				Entries:  fleet("1"),
				Archives: fleet("1", "2"),
			},
			wantErr: []string{
				"fleet 1 is not in index.yaml yet but already has an archive in assets/",
				"assets/ has an archive of fleet 2 but there is no entry for it",
			},
		},
		{
			name: "a published version lost its archive",
			state: State{
				Entries:   fleet("1"),
				Published: fleet("1"),
			},
			wantErr: []string{"fleet 1 is in index.yaml but has no archive in assets/"},
		},
		{
			name: "a new entry that already has an archive, which is also a hidden version that is no longer hidden",
			state: State{
				Entries:  fleet("1"),
				Archives: fleet("1"),
			},
			wantErr: []string{"fleet 1 is not in index.yaml yet but already has an archive in assets/"},
		},
		{
			name: "a hidden version that was never published",
			state: State{
				Entries: fleet("1"),
				Hidden:  fleet("1"),
			},
			wantErr: []string{"fleet 1 is hidden but was never published: it is not in index.yaml and has no archive in assets/"},
		},
		{
			name: "a hidden version that lost its archive",
			state: State{
				Entries:   fleet("1"),
				Hidden:    fleet("1"),
				Published: fleet("1"),
			},
			wantErr: []string{"fleet 1 is in index.yaml but has no archive in assets/"},
		},
		{
			name: "every version at fault is named",
			state: State{
				Entries:   fleet("1", "3"),
				Published: fleet("1", "2", "3"),
				Archives:  fleet("1", "2"),
			},
			wantErr: []string{
				"fleet 2 is in index.yaml but has no entry: entries are never removed",
				"fleet 3 is in index.yaml but has no archive in assets/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.state.Plan()

			require.Error(t, err)
			require.Equal(t, Plan{}, got)
			for _, msg := range tt.wantErr {
				require.ErrorContains(t, err, msg)
			}
		})
	}
}
