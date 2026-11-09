package oci

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Policy says what may be pushed where on the SUSE registries.
type Policy struct {
	// Images are the host/path repositories image dependencies go to.
	Images []string
	// Charts are the host/path repositories the charts of each edition go to.
	Charts map[string][]string
	// Editions maps every listed chart to its edition. Anything else that is
	// pushed is an image dependency.
	Editions map[string]string
}

// Guard applies a Policy. The hosts of the Policy are the SUSE registries:
// anywhere else, the Guard allows everything.
type Guard struct {
	hosts    map[string]bool
	images   []Target
	charts   map[string][]Target
	editions map[string]string
}

// NewGuard returns a Guard for p. Every repository must be a host[:port]/path.
func NewGuard(p Policy) (*Guard, error) {
	if len(p.Images) == 0 || len(p.Charts) == 0 {
		return nil, errors.New("guard: no image or chart repositories")
	}

	g := &Guard{
		hosts:    make(map[string]bool),
		charts:   make(map[string][]Target, len(p.Charts)),
		editions: maps.Clone(p.Editions),
	}
	var err error
	if g.images, err = g.parseAll(p.Images); err != nil {
		return nil, err
	}
	for edition, repositories := range p.Charts {
		if g.charts[edition], err = g.parseAll(repositories); err != nil {
			return nil, err
		}
	}
	return g, nil
}

// parseAll normalizes repositories and records their hosts.
func (g *Guard) parseAll(repositories []string) ([]Target, error) {
	targets := make([]Target, 0, len(repositories))
	for _, repository := range repositories {
		t, err := ParseTarget(repository)
		if err != nil {
			return nil, fmt.Errorf("guard: %w", err)
		}
		if t.Path == "" {
			return nil, fmt.Errorf("guard: repository %q has no path", repository)
		}
		t = Target{Host: normalizeHost(t.Host), Path: strings.ToLower(t.Path)}
		g.hosts[t.Host] = true
		targets = append(targets, t)
	}
	return targets, nil
}

// Guarded reports whether t is on a SUSE registry.
func (g *Guard) Guarded(t Target) bool {
	return g.hosts[normalizeHost(t.Host)]
}

// Check refuses pushing the chart or image called name to t when t is on a SUSE
// registry and the policy does not allow it. A listed chart goes to the
// repositories of its edition, and only there. Anything else is an image
// dependency and goes to an image repository, never to a chart repository. It
// looks at the address only and makes no network call.
func (g *Guard) Check(t Target, name string) error {
	if !g.Guarded(t) {
		return nil
	}

	where := Target{Host: normalizeHost(t.Host), Path: strings.ToLower(t.Path)}
	if edition, listed := g.editions[name]; listed {
		return g.checkChart(t, where, name, edition)
	}
	return g.checkImage(where, name)
}

// checkChart allows a chart in a repository of its edition.
func (g *Guard) checkChart(t, where Target, name, edition string) error {
	if slices.Contains(g.charts[edition], where) {
		return nil
	}

	for other, repositories := range g.charts {
		if other != edition && slices.Contains(repositories, where) {
			return refused("%s is a %s chart and cannot go to %s, which is for %s charts", name, edition, location(t), other)
		}
	}
	return refused("%s is a %s chart and can only go to %s", name, edition, listRepositories(g.charts[edition], where.Host))
}

// checkImage allows an image dependency in an image repository, as long as its
// final path is not a chart repository.
func (g *Guard) checkImage(where Target, name string) error {
	if !slices.Contains(g.images, where) {
		return refused("%s is not a chart listed in the rules file, so it is an image and can only go to %s",
			name, listRepositories(g.images, where.Host))
	}

	final := Target{Host: where.Host, Path: where.Path + "/" + strings.ToLower(name)}
	for _, repositories := range g.charts {
		if slices.Contains(repositories, final) {
			return refused("%s is not a chart listed in the rules file, so it cannot go to the chart repository %s", name, location(final))
		}
	}
	return nil
}

// refused is the error for a push the policy forbids.
func refused(format string, args ...any) error {
	return fmt.Errorf("pushing to this registry is not allowed: %s", fmt.Sprintf(format, args...))
}

// location is t as host/path.
func location(t Target) string {
	return t.Host + "/" + t.Path
}

// listRepositories names the repositories on host, for an error message.
func listRepositories(repositories []Target, host string) string {
	var names []string
	for _, r := range repositories {
		if r.Host == host {
			names = append(names, location(r))
		}
	}
	return strings.Join(names, " or ")
}

// normalizeHost lower-cases host and drops the port and a trailing dot, so
// that REGISTRY.suse.com., registry.suse.com:443 and registry.suse.com are
// all the same registry.
func normalizeHost(host string) string {
	name, _, _ := strings.Cut(host, ":")
	return strings.TrimSuffix(strings.ToLower(name), ".")
}
