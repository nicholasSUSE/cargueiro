// Package ship prepares what a chart needs to be shipped to a charts repository:
// its entry and its logo.
package ship

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

// maxLogoSize is the most that is read of a logo: Rancher reads at most this
// much of an icon, so a larger one would be cut off there.
const maxLogoSize = 1 << 20

// Logo is a logo that was downloaded.
type Logo struct {
	// Data is the content of the file.
	Data []byte
	// Extension is the extension of the file, with its dot, such as ".svg".
	Extension string
}

// FetchLogo downloads the logo at address, which must be an http or https
// address. It fails when the server does not answer 200 with an image of a known
// type, or when the logo is empty or larger than Rancher reads.
func FetchLogo(ctx context.Context, client *http.Client, address string) (Logo, error) {
	if u, err := url.Parse(address); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Logo{}, fmt.Errorf("the icon %q must be an http or https address", address)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, http.NoBody)
	if err != nil {
		return Logo{}, fmt.Errorf("download the icon %s: %w", address, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Logo{}, fmt.Errorf("download the icon %s: %w", address, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Logo{}, fmt.Errorf("download the icon %s: the server answered %d", address, resp.StatusCode)
	}
	extension, err := extensionOf(address, resp.Header.Get("Content-Type"))
	if err != nil {
		return Logo{}, err
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxLogoSize+1))
	if err != nil {
		return Logo{}, fmt.Errorf("download the icon %s: %w", address, err)
	}
	switch {
	case len(data) == 0:
		return Logo{}, fmt.Errorf("the icon %s is empty", address)
	case len(data) > maxLogoSize:
		return Logo{}, fmt.Errorf("the icon %s is larger than %d bytes, which is all that Rancher reads", address, maxLogoSize)
	}
	return Logo{Data: data, Extension: extension}, nil
}

// extensionOf returns the extension of a logo from its content type.
func extensionOf(address, contentType string) (string, error) {
	if contentType == "" {
		return "", fmt.Errorf("the icon %s has no content type, so it cannot be named", address)
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", fmt.Errorf("the icon %s has the content type %q: %w", address, contentType, err)
	}
	extension, found := logoExtension(strings.ToLower(mediaType))
	if !found {
		return "", fmt.Errorf("the icon %s has the content type %q, which is not one of the image types that are kept", address, mediaType)
	}
	return extension, nil
}

// logoExtension returns the extension, with its dot, of the file a logo of this
// media type is kept in. A fixed list gives the same name on every machine.
func logoExtension(mediaType string) (string, bool) {
	switch mediaType {
	case "image/svg+xml":
		return ".svg", true
	case "image/png":
		return ".png", true
	case "image/jpeg":
		return ".jpg", true
	case "image/gif":
		return ".gif", true
	case "image/webp":
		return ".webp", true
	case "image/x-icon", "image/vnd.microsoft.icon":
		return ".ico", true
	}
	return "", false
}
