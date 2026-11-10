package ship

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// logoServer answers every request with the given status, content type and body.
func logoServer(t *testing.T, status int, contentType string, body []byte) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		} else {
			w.Header()["Content-Type"] = nil // none, and none guessed by net/http
		}
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchLogo(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		wantExt     string
	}{
		{
			name:        "svg",
			contentType: "image/svg+xml",
			wantExt:     ".svg",
		},
		{
			name:        "svg with a charset",
			contentType: "image/svg+xml; charset=utf-8",
			wantExt:     ".svg",
		},
		{
			name:        "png",
			contentType: "image/png",
			wantExt:     ".png",
		},
		{
			name:        "jpeg is a jpg",
			contentType: "image/jpeg",
			wantExt:     ".jpg",
		},
		{
			name:        "gif",
			contentType: "image/gif",
			wantExt:     ".gif",
		},
		{
			name:        "webp",
			contentType: "image/webp",
			wantExt:     ".webp",
		},
		{
			name:        "an icon, by either of its names",
			contentType: "image/x-icon",
			wantExt:     ".ico",
		},
		{
			name:        "an icon, by the other name",
			contentType: "image/vnd.microsoft.icon",
			wantExt:     ".ico",
		},
		{
			name:        "the type in capitals",
			contentType: "IMAGE/PNG",
			wantExt:     ".png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := logoServer(t, http.StatusOK, tt.contentType, []byte("the logo"))

			got, err := FetchLogo(context.Background(), srv.Client(), srv.URL+"/logo")

			require.NoError(t, err)
			require.Equal(t, Logo{Data: []byte("the logo"), Extension: tt.wantExt}, got)
		})
	}
}

func TestFetchLogoFails(t *testing.T) {
	tests := []struct {
		name    string
		server  func(t *testing.T) string
		wantErr string
	}{
		{
			name: "the server says not found",
			server: func(t *testing.T) string {
				return logoServer(t, http.StatusNotFound, "image/png", []byte("nope")).URL
			},
			wantErr: "answered 404",
		},
		{
			name: "the server fails",
			server: func(t *testing.T) string {
				return logoServer(t, http.StatusInternalServerError, "image/png", nil).URL
			},
			wantErr: "answered 500",
		},
		{
			name: "no content type",
			server: func(t *testing.T) string {
				return logoServer(t, http.StatusOK, "", []byte("the logo")).URL
			},
			wantErr: "has no content type",
		},
		{
			name: "a page and not an image",
			server: func(t *testing.T) string {
				return logoServer(t, http.StatusOK, "text/html", []byte("<html/>")).URL
			},
			wantErr: `has the content type "text/html", which is not one of the image types that are kept`,
		},
		{
			name: "a content type that is not valid",
			server: func(t *testing.T) string {
				return logoServer(t, http.StatusOK, "image/png; charset", []byte("x")).URL
			},
			wantErr: "content type",
		},
		{
			name: "nothing in the body",
			server: func(t *testing.T) string {
				return logoServer(t, http.StatusOK, "image/png", nil).URL
			},
			wantErr: "is empty",
		},
		{
			name: "a logo larger than the one Rancher reads",
			server: func(t *testing.T) string {
				return logoServer(t, http.StatusOK, "image/png", bytes.Repeat([]byte("x"), maxLogoSize+1)).URL
			},
			wantErr: "larger than 1048576 bytes",
		},
		{
			name: "a server that is not there",
			server: func(t *testing.T) string {
				srv := logoServer(t, http.StatusOK, "image/png", []byte("x"))
				srv.Close()
				return srv.URL
			},
			wantErr: "download",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.server(t)

			got, err := FetchLogo(context.Background(), http.DefaultClient, url+"/logo.png")

			require.ErrorContains(t, err, tt.wantErr)
			require.Equal(t, Logo{}, got)
		})
	}
}

func TestFetchLogoRefusesWhatIsNotAWebAddress(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "a local file",
			url:  "file://assets/logos/fleet.svg",
		},
		{
			name: "ftp",
			url:  "ftp://example.com/fleet.svg",
		},
		{
			name: "no scheme",
			url:  "example.com/fleet.svg",
		},
		{
			name: "nothing",
			url:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FetchLogo(context.Background(), http.DefaultClient, tt.url)

			require.ErrorContains(t, err, "must be an http or https address")
			require.Equal(t, Logo{}, got)
		})
	}
}
