package cmd

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// fakeLister serves the tags it is given and records what it was asked.
type fakeLister struct {
	tags []string
	err  error

	called bool
	repo   oci.ChartRepo
	opts   oci.FetchOptions
}

func (f *fakeLister) list(_ context.Context, repo oci.ChartRepo, opts oci.FetchOptions) ([]string, error) {
	f.called = true
	f.repo, f.opts = repo, opts
	return f.tags, f.err
}

// runList executes list with the rules in checkRules.
func runList(t *testing.T, fake *fakeLister, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{listTags: fake.list})
	require.NoError(t, err)
	root.SetArgs(append([]string{"list"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestListPrintsTheNewestFirst(t *testing.T) {
	fake := &fakeLister{tags: []string{
		"111.0.3_up0.16.3-rc.1",
		"111.0.3_up0.16.3",
		"111.0.10_up0.16.3",
		"111.0.9_up0.16.3",
		"111.0.3_up0.16.10",
	}}

	t.Run("text", func(t *testing.T) {
		stdout, stderr, err := runList(t, fake, "registry.example.com/rancher/fleet")

		require.NoError(t, err)
		require.Empty(t, stderr)
		require.Equal(t, `111.0.10+up0.16.3
111.0.9+up0.16.3
111.0.3+up0.16.10
111.0.3+up0.16.3
111.0.3+up0.16.3-rc.1
`, stdout)
	})

	t.Run("json", func(t *testing.T) {
		stdout, _, err := runList(t, fake, "registry.example.com/rancher/fleet", "-o", "json")

		require.NoError(t, err)
		require.JSONEq(t, `[
			{"version": "111.0.10+up0.16.3", "tag": "111.0.10_up0.16.3"},
			{"version": "111.0.9+up0.16.3", "tag": "111.0.9_up0.16.3"},
			{"version": "111.0.3+up0.16.10", "tag": "111.0.3_up0.16.10"},
			{"version": "111.0.3+up0.16.3", "tag": "111.0.3_up0.16.3"},
			{"version": "111.0.3+up0.16.3-rc.1", "tag": "111.0.3_up0.16.3-rc.1"}
		]`, stdout)
	})
}

func TestListPlainChart(t *testing.T) {
	fake := &fakeLister{tags: []string{"111.0.0", "111.0.10", "111.0.9"}}

	stdout, _, err := runList(t, fake, "registry.example.com/rancher/system-upgrade-controller")

	require.NoError(t, err)
	require.Equal(t, "111.0.10\n111.0.9\n111.0.0\n", stdout)
}

func TestListWithoutTags(t *testing.T) {
	fake := &fakeLister{}

	t.Run("text prints nothing", func(t *testing.T) {
		stdout, stderr, err := runList(t, fake, "registry.example.com/rancher/fleet")

		require.NoError(t, err)
		require.Empty(t, stdout)
		require.Empty(t, stderr)
	})

	t.Run("json prints an empty list", func(t *testing.T) {
		stdout, _, err := runList(t, fake, "registry.example.com/rancher/fleet", "-o", "json")

		require.NoError(t, err)
		require.Equal(t, "[]\n", stdout)
	})
}

func TestListAsksForTheRepository(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want oci.ChartRepo
		opts oci.FetchOptions
	}{
		{
			name: "a repository",
			args: []string{"registry.example.com/rancher/catalog/fleet"},
			want: oci.ChartRepo{Host: "registry.example.com", Repository: "rancher/catalog/fleet"},
			opts: oci.FetchOptions{PlainHTTP: false},
		},
		{
			name: "the scheme",
			args: []string{"oci://registry.example.com/rancher/fleet"},
			want: oci.ChartRepo{Host: "registry.example.com", Repository: "rancher/fleet"},
			opts: oci.FetchOptions{PlainHTTP: false},
		},
		{
			name: "plain HTTP",
			args: []string{"127.0.0.1:5050/rancher/fleet", "--plain-http"},
			want: oci.ChartRepo{Host: "127.0.0.1:5050", Repository: "rancher/fleet"},
			opts: oci.FetchOptions{PlainHTTP: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeLister{}

			_, _, err := runList(t, fake, tt.args...)

			require.NoError(t, err)
			require.Equal(t, tt.want, fake.repo)
			require.Equal(t, tt.opts, fake.opts)
		})
	}
}

func TestListFailsWithoutPrinting(t *testing.T) {
	tests := []struct {
		name       string
		fake       *fakeLister
		args       []string
		wantErr    string
		wantCalled bool
	}{
		{
			name:       "the registry fails",
			fake:       &fakeLister{err: errors.New("registry.example.com/rancher/fleet was not found in the registry")},
			args:       []string{"registry.example.com/rancher/fleet"},
			wantErr:    "was not found in the registry",
			wantCalled: true,
		},
		{
			name:    "a chart that is not in the rules file",
			fake:    &fakeLister{},
			args:    []string{"registry.example.com/rancher/epinio"},
			wantErr: `chart "epinio" is not listed in the rules file`,
		},
		{
			name:    "a reference with a tag",
			fake:    &fakeLister{},
			args:    []string{"registry.example.com/rancher/fleet:111.0.3_up0.16.3"},
			wantErr: "a tag is not accepted",
		},
		{
			name:    "an unknown output format",
			fake:    &fakeLister{},
			args:    []string{"registry.example.com/rancher/fleet", "-o", "yaml"},
			wantErr: `unknown output format "yaml"`,
		},
		{
			name:       "a tag that is not a version",
			fake:       &fakeLister{tags: []string{"111.0.3_up0.16.3", "latest"}},
			args:       []string{"registry.example.com/rancher/fleet"},
			wantErr:    `order the versions of registry.example.com/rancher/fleet: version "latest" must look like`,
			wantCalled: true,
		},
		{
			name:       "a version without the +up part",
			fake:       &fakeLister{tags: []string{"111.0.3"}},
			args:       []string{"registry.example.com/rancher/fleet"},
			wantErr:    `version "111.0.3" must look like`,
			wantCalled: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runList(t, tt.fake, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
			require.Equal(t, tt.wantCalled, tt.fake.called, "the registry is read only when the arguments are good")
		})
	}
}
