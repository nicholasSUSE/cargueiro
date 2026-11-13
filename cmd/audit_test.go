package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rancher/cargueiro/internal/audit"
	"github.com/rancher/cargueiro/internal/oci"
)

const (
	auditManifest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	auditArchive  = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

// fakeAuditRegistry answers every read with the same manifest, and records the last one.
type fakeAuditRegistry struct {
	info oci.ManifestInfo
	err  error

	ref  oci.ChartRef
	opts oci.FetchOptions
}

func (f *fakeAuditRegistry) fetch(_ context.Context, ref oci.ChartRef, opts oci.FetchOptions) (oci.ManifestInfo, error) {
	f.ref, f.opts = ref, opts
	return f.info, f.err
}

// auditRepo writes a repository whose index has fleet 111.0.3+up0.16.3.
func auditRepo(t *testing.T, pinned string) string {
	t.Helper()

	dir := t.TempDir()
	text := "apiVersion: v1\nentries:\n  fleet:\n  - apiVersion: v2\n    name: fleet\n    version: 111.0.3+up0.16.3\n    digest: " + auditArchive +
		"\n    urls:\n    - oci://stgregistry.suse.com/rancher/catalog/fleet:111.0.3_up0.16.3@" + pinned + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.yaml"), []byte(text), 0o600))
	return dir
}

func runAudit(t *testing.T, fake *fakeAuditRegistry, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var out, errOut bytes.Buffer
	opts := &rootOptions{
		stderr:    &errOut,
		getenv:    func(string) string { return "" },
		openStore: openerFor(newTestStore(t)),
		prompt:    &fakePrompter{},
		rules:     []byte(checkRules),
	}
	root, err := newRootCmd(&out, &errOut, opts, dependencies{auditFetch: fake.fetch})
	require.NoError(t, err)
	root.SetArgs(append([]string{"audit"}, args...))

	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestAuditPrintsTheReport(t *testing.T) {
	fake := &fakeAuditRegistry{info: oci.ManifestInfo{ManifestDigest: auditManifest, ChartDigest: "sha256:" + auditArchive}}
	dir := auditRepo(t, auditManifest)

	t.Run("text", func(t *testing.T) {
		stdout, _, err := runAudit(t, fake, "--repo", dir)

		require.NoError(t, err)
		require.Contains(t, stdout, "check: urls\nvalid: true\n")
		require.Contains(t, stdout, "check: manifest\nvalid: true\n")
		require.Contains(t, stdout, "check: archive\nvalid: true\n")
		require.Contains(t, stdout, "error: no-error\n")
		require.Equal(t, oci.ChartRef{Host: "stgregistry.suse.com", Repository: "rancher/catalog/fleet", Tag: "111.0.3_up0.16.3"}, fake.ref)
	})

	t.Run("json", func(t *testing.T) {
		stdout, _, err := runAudit(t, fake, "--repo", dir, "--output", "json")

		require.NoError(t, err)
		var got []map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &got))
		require.Len(t, got, 3)
		require.Equal(t, "urls", got[0]["check"])
		require.Equal(t, true, got[0]["valid"])
		require.Equal(t, "no-error", got[0]["error"])
		require.NotEmpty(t, got[0]["description"])
	})
}

func TestAuditExitsWithAnErrorWhenACheckFails(t *testing.T) {
	fake := &fakeAuditRegistry{info: oci.ManifestInfo{ManifestDigest: "sha256:" + string(bytes.Repeat([]byte("b"), 64)), ChartDigest: "sha256:" + auditArchive}}

	stdout, _, err := runAudit(t, fake, "--repo", auditRepo(t, auditManifest))

	require.EqualError(t, err, "the repository is not valid")
	require.Contains(t, stdout, "check: manifest\nvalid: false\n")
	require.Contains(t, stdout, "check: urls\nvalid: true\n", "the other checks are in the report")
	require.Contains(t, stdout, "error: fleet 111.0.3+up0.16.3: the tag 111.0.3_up0.16.3")
}

func TestAuditReportsARegistryThatFailed(t *testing.T) {
	fake := &fakeAuditRegistry{err: errors.New("connection refused")}

	stdout, _, err := runAudit(t, fake, "--repo", auditRepo(t, auditManifest), "--registry", "127.0.0.1:5052/e2e", "--plain-http")

	require.Error(t, err)
	require.Contains(t, stdout, "connection refused")
	require.Equal(t, "127.0.0.1:5052", fake.ref.Host)
	require.True(t, fake.opts.PlainHTTP)
}

func TestAuditFails(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no repository", wantErr: "--repo is required"},
		{name: "unknown format", args: []string{"--repo", ".", "--output", "xml"}, wantErr: `unknown output format "xml"`},
		{name: "an argument", args: []string{"--repo", ".", "extra"}, wantErr: "unknown command"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := runAudit(t, &fakeAuditRegistry{}, tt.args...)

			require.ErrorContains(t, err, tt.wantErr)
			require.Empty(t, stdout)
		})
	}
}

func TestAuditTextPrintsOneProblemToALine(t *testing.T) {
	got := auditText(audit.Report{{Check: "manifest", Description: "d", Error: "a\nb"}})

	require.Equal(t, "check: manifest\nvalid: false\ndescription: d\nerror:\n  a\n  b\n", got)
}
