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

func TestAdded(t *testing.T) {
	tests := []struct {
		name  string
		state State
		want  []Ref
	}{
		{
			name:  "nothing at all",
			state: State{},
			want:  nil,
		},
		{
			name: "every version is already published",
			state: State{
				Entries:   fleet("1", "2"),
				Published: fleet("1", "2"),
				Archives:  fleet("1", "2"),
			},
			want: nil,
		},
		{
			name: "an entry that is not published yet",
			state: State{
				Entries:   fleet("1", "2"),
				Published: fleet("1"),
				Archives:  fleet("1"),
			},
			want: fleet("2"),
		},
		{
			name: "everything is new on the first run",
			state: State{
				Entries: fleet("2", "1"),
			},
			want: fleet("1", "2"),
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
			want: []Ref{
				{Chart: "alerting", Version: "9"},
				{Chart: "fleet", Version: "1"},
				{Chart: "fleet", Version: "2"},
			},
		},
		{
			name: "the same version listed twice is one version",
			state: State{
				Entries: fleet("1", "1"),
			},
			want: fleet("1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.state.Added()

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestAddedFails(t *testing.T) {
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
			name: "a new entry that already has an archive",
			state: State{
				Entries:  fleet("1"),
				Archives: fleet("1"),
			},
			wantErr: []string{"fleet 1 is not in index.yaml yet but already has an archive in assets/"},
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
			got, err := tt.state.Added()

			require.Error(t, err)
			require.Nil(t, got)
			for _, msg := range tt.wantErr {
				require.ErrorContains(t, err, msg)
			}
		})
	}
}
