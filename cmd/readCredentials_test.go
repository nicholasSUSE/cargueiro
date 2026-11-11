package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// runListWith executes list with the given environment.
func runListWith(t *testing.T, fake *fakeLister, env map[string]string, args ...string) error {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(k string) string { return env[k] },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{listTags: fake.list})
	require.NoError(t, err)
	root.SetArgs(append([]string{"list"}, args...))
	return root.Execute()
}

func TestReadsTakeCredentials(t *testing.T) {
	const repo = "registry.example.com/rancher/fleet"

	tests := []struct {
		name         string
		env          map[string]string
		args         []string
		wantUsername string
		wantPassword string
		wantErr      string
	}{
		{
			name: "none stays anonymous",
			args: []string{repo},
		},
		{
			name:         "username flag and password from the environment",
			env:          map[string]string{envPassword: "from-env"},
			args:         []string{repo, "--username", "reader"},
			wantUsername: "reader",
			wantPassword: "from-env",
		},
		{
			name:         "both from the environment",
			env:          map[string]string{envUsername: "reader", envPassword: "from-env"},
			args:         []string{repo},
			wantUsername: "reader",
			wantPassword: "from-env",
		},
		{
			name:    "a username without a password",
			args:    []string{repo, "--username", "reader"},
			wantErr: "a password is required",
		},
		{
			name:    "a password without a username",
			env:     map[string]string{envPassword: "from-env"},
			args:    []string{repo},
			wantErr: "a username is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLister{tags: []string{"111.0.3_up0.16.3"}}

			err := runListWith(t, fake, tt.env, tt.args...)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.False(t, fake.called, "nothing is read when the credentials are incomplete")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantUsername, fake.opts.Username)
			require.Equal(t, tt.wantPassword, fake.opts.Password)
		})
	}
}
