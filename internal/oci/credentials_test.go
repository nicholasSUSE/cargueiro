package oci

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testUser     = "reader"
	testPassword = "s3cret-pass"
)

// authLog records the Authorization header of every request a server gets.
type authLog struct {
	mu      sync.Mutex
	headers []string
}

func (l *authLog) add(h string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.headers = append(l.headers, h)
}

// sent returns the non-empty Authorization headers seen.
func (l *authLog) sent() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	var got []string
	for _, h := range l.headers {
		if h != "" {
			got = append(got, h)
		}
	}
	return got
}

// requireBasic answers 401 with a Basic challenge until the request carries the
// right username and password. Unless user is empty, in which case it lets
// everything through.
func requireBasic(next http.Handler, user, password string, seen *authLog) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.add(r.Header.Get("Authorization"))
		if user != "" {
			u, p, ok := r.BasicAuth()
			if !ok || u != user || p != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="registry"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// serveWithAuth starts a registry behind requireBasic and returns the reference
// of the test chart on it.
func serveWithAuth(t *testing.T, f *fakeRegistry, user, password string, seen *authLog) ChartRef {
	t.Helper()
	srv := httptest.NewServer(requireBasic(f, user, password, seen))
	t.Cleanup(srv.Close)
	return ChartRef{
		Host:       strings.TrimPrefix(srv.URL, "http://"),
		Repository: testRepository,
		Tag:        testTag,
	}
}

func TestReadsWithCredentials(t *testing.T) {
	reads := []struct {
		name string
		read func(ref ChartRef, pinned string, opts FetchOptions) error
	}{
		{
			name: "FetchChart",
			read: func(ref ChartRef, _ string, opts FetchOptions) error {
				_, err := FetchChart(context.Background(), ref, opts)
				return err
			},
		},
		{
			name: "ResolveDigest",
			read: func(ref ChartRef, _ string, opts FetchOptions) error {
				_, err := ResolveDigest(context.Background(), ref, opts)
				return err
			},
		},
		{
			name: "PullArchive",
			read: func(ref ChartRef, pinned string, opts FetchOptions) error {
				_, err := PullArchive(context.Background(), ChartRepo{Host: ref.Host, Repository: ref.Repository}, pinned, opts)
				return err
			},
		},
	}

	for _, r := range reads {
		t.Run(r.name+" with the right credentials", func(t *testing.T) {
			f := newFakeRegistry()
			f.addChart(t, validArchive(t))
			pinned := f.pin(t)
			seen := &authLog{}
			ref := serveWithAuth(t, f, testUser, testPassword, seen)

			err := r.read(ref, pinned, FetchOptions{PlainHTTP: true, Username: testUser, Password: testPassword})

			require.NoError(t, err)
			require.NotEmpty(t, seen.sent(), "the credentials reached the registry")
		})

		t.Run(r.name+" with the wrong password", func(t *testing.T) {
			f := newFakeRegistry()
			f.addChart(t, validArchive(t))
			pinned := f.pin(t)
			ref := serveWithAuth(t, f, testUser, testPassword, &authLog{})

			err := r.read(ref, pinned, FetchOptions{PlainHTTP: true, Username: testUser, Password: "wrong-pass"})

			require.ErrorContains(t, err, "refused the credentials")
			require.NotContains(t, err.Error(), "wrong-pass")
			require.NotContains(t, err.Error(), testPassword)
		})

		t.Run(r.name+" without credentials", func(t *testing.T) {
			f := newFakeRegistry()
			f.addChart(t, validArchive(t))
			pinned := f.pin(t)
			ref := serveWithAuth(t, f, testUser, testPassword, &authLog{})

			err := r.read(ref, pinned, FetchOptions{PlainHTTP: true})

			require.ErrorContains(t, err, "granted no access")
			require.ErrorContains(t, err, "without credentials")
			require.ErrorContains(t, err, "registry login")
		})

		t.Run(r.name+" on an anonymous registry stays anonymous", func(t *testing.T) {
			f := newFakeRegistry()
			f.addChart(t, validArchive(t))
			pinned := f.pin(t)
			seen := &authLog{}
			ref := serveWithAuth(t, f, "", "", seen)

			err := r.read(ref, pinned, FetchOptions{PlainHTTP: true})

			require.NoError(t, err)
			require.Empty(t, seen.sent(), "no credentials are sent when none are given")
		})
	}
}

func TestListTagsWithCredentials(t *testing.T) {
	newRegistry := func(t *testing.T, seen *authLog) ChartRepo {
		srv := httptest.NewServer(requireBasic(&tagRegistry{repository: testRepository, tags: []string{"1.0.0"}}, testUser, testPassword, seen))
		t.Cleanup(srv.Close)
		return ChartRepo{Host: strings.TrimPrefix(srv.URL, "http://"), Repository: testRepository}
	}

	t.Run("with the right credentials", func(t *testing.T) {
		repo := newRegistry(t, &authLog{})

		tags, err := ListTags(context.Background(), repo, FetchOptions{PlainHTTP: true, Username: testUser, Password: testPassword})

		require.NoError(t, err)
		require.Equal(t, []string{"1.0.0"}, tags)
	})

	t.Run("without credentials", func(t *testing.T) {
		repo := newRegistry(t, &authLog{})

		_, err := ListTags(context.Background(), repo, FetchOptions{PlainHTTP: true})

		require.ErrorContains(t, err, "granted no access")
	})
}

func TestCredentialsStayWithTheRegistry(t *testing.T) {
	t.Run("a redirect to another host does not carry them", func(t *testing.T) {
		f := newFakeRegistry()
		layer := f.addChart(t, validArchive(t))

		// Registries redirect blob downloads to a storage host.
		storageSeen := &authLog{}
		storage := httptest.NewServer(requireBasic(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(validArchive(t))
		}), "", "", storageSeen))
		t.Cleanup(storage.Close)

		seen := &authLog{}
		srv := httptest.NewServer(requireBasic(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == blobPath(layer) {
				http.Redirect(w, r, storage.URL+"/blob", http.StatusTemporaryRedirect)
				return
			}
			f.ServeHTTP(w, r)
		}), testUser, testPassword, seen))
		t.Cleanup(srv.Close)
		ref := ChartRef{Host: strings.TrimPrefix(srv.URL, "http://"), Repository: testRepository, Tag: testTag}

		_, err := FetchChart(context.Background(), ref, FetchOptions{PlainHTTP: true, Username: testUser, Password: testPassword})

		require.NoError(t, err)
		require.NotEmpty(t, seen.sent(), "the registry got the credentials")
		require.Empty(t, storageSeen.sent(), "the storage host did not")
	})

	t.Run("none are sent over HTTP unless it was asked for", func(t *testing.T) {
		f := newFakeRegistry()
		f.addChart(t, validArchive(t))
		seen := &authLog{}
		ref := serveWithAuth(t, f, testUser, testPassword, seen)

		_, err := FetchChart(context.Background(), ref, FetchOptions{Username: testUser, Password: testPassword})

		require.Error(t, err)
		require.Empty(t, seen.sent())
	})
}

func TestFetchErrorForAReadWithCredentials(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			err := fetchError("registry.example.com/rancher/catalog/fleet:1.0.0", statusError(status), true)

			require.EqualError(t, err, "the registry refused the credentials for registry.example.com/rancher/catalog/fleet:1.0.0: check the username and the password, and that the account may read this repository")
		})
	}
}
