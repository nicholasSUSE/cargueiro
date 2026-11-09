package oci

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	ci "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
)

// repoName is the form OCI allows for a repository path component.
var repoName = regexp.MustCompile(`^[a-z0-9]+([._-][a-z0-9]+)*$`)

// Client uploads a packaged chart to the registry at ref and returns the
// reference it was stored under.
type Client interface {
	Push(data []byte, ref string) (string, error)
}

// Pusher pushes chart archives to one registry target.
type Pusher struct {
	client Client
	target Target
	guard  *Guard
}

// NewPusher returns a Pusher that uploads through client to target, unless
// guard forbids it.
func NewPusher(client Client, target Target, guard *Guard) *Pusher {
	return &Pusher{client: client, target: target, guard: guard}
}

// Push uploads the chart archive at archivePath to <target>/<name>:<version>,
// taking name and version from the chart's own metadata. It returns the
// reference the chart was stored under.
func (p *Pusher) Push(ctx context.Context, archivePath string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(archivePath)
	if err != nil {
		return "", fmt.Errorf("read archive %s: %w", archivePath, err)
	}

	name, version, err := chartIdentity(data)
	if err != nil {
		return "", fmt.Errorf("invalid chart archive: %s: %w", archivePath, err)
	}

	if err := p.guard.Check(p.target, name); err != nil {
		return "", err
	}

	ref, err := p.client.Push(data, p.target.Ref(name, version))
	if err != nil {
		return "", fmt.Errorf("push %s: %w", archivePath, err)
	}

	return ref, nil
}

// ReadIdentity returns the name and version stored in the chart archive at
// archivePath.
func ReadIdentity(archivePath string) (name, version string, err error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return "", "", fmt.Errorf("read archive %s: %w", archivePath, err)
	}
	name, version, err = chartIdentity(data)
	if err != nil {
		return "", "", fmt.Errorf("invalid chart archive: %s: %w", archivePath, err)
	}
	return name, version, nil
}

// chartIdentity returns the name and version stored in a chart archive.
func chartIdentity(data []byte) (name, version string, err error) {
	loaded, err := loader.LoadArchive(bytes.NewReader(data))
	if err != nil {
		return "", "", err
	}

	acc, err := ci.NewAccessor(loaded)
	if err != nil {
		return "", "", err
	}

	name = acc.Name()
	version, _ = acc.MetadataAsMap()["Version"].(string)

	if !repoName.MatchString(name) {
		return "", "", fmt.Errorf("chart name %q is not a valid registry repository name", name)
	}
	if version == "" {
		return "", "", errors.New("chart has no version")
	}
	return name, version, nil
}
