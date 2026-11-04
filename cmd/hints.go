package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nicholasSUSE/cargueiro/internal/oci"
)

// pushContext is what the push flags say, used to word advice precisely.
type pushContext struct {
	username string
	caFile   string
}

// hintFor returns one line of advice for a push failure with a recognized
// cause, or "" when there is nothing useful to add.
func hintFor(err error, ctx pushContext) string {
	var registryErr *oci.RegistryError
	if !errors.As(err, &registryErr) {
		return ""
	}

	switch registryErr.Kind {
	case oci.KindAuthRequired:
		return "the registry requires credentials: give --username (or $" + envUsername +
			") and set $" + envPassword + " or use --password-stdin"
	case oci.KindAuthRejected:
		return fmt.Sprintf("the registry rejected the credentials of user %q: check the username and the password", ctx.username)
	case oci.KindForbidden:
		if ctx.username == "" {
			return "the registry does not allow anonymous pushes: give --username and a password"
		}
		return fmt.Sprintf("user %q is not allowed to push to this repository: check its permissions on the --registry path", ctx.username)
	case oci.KindTLSRequired:
		return "the registry answered plain HTTP with 400, so it probably serves HTTPS: drop --plain-http" +
			" (add --ca-file if its certificate is self-signed)"
	case oci.KindHTTPOnly:
		return "the registry answers plain HTTP: use --plain-http (local testing only)"
	case oci.KindUntrustedCertificate:
		if ctx.caFile != "" {
			return fmt.Sprintf("the registry's certificate is not signed by a CA in %s: pass the right CA with --ca-file", ctx.caFile)
		}
		return "the registry's certificate is not trusted: pass the CA that signed it with --ca-file"
	case oci.KindCertificateName:
		return "the registry's certificate is not valid for the host in --registry: use the host name the certificate was issued for"
	case oci.KindUnreachable:
		return "nothing accepted the connection: check the host and port in --registry and that the registry is running"
	case oci.KindUnknownHost:
		return "the host name in --registry could not be resolved: check it for typos"
	case oci.KindTimeout:
		return "the registry did not answer in time: check the network and the address in --registry"
	}
	return ""
}

// hintSet collects distinct hints in the order they were first seen.
type hintSet struct {
	seen  map[string]bool
	order []string
}

func (s *hintSet) add(hint string) {
	if hint == "" || s.seen[hint] {
		return
	}
	if s.seen == nil {
		s.seen = make(map[string]bool)
	}
	s.seen[hint] = true
	s.order = append(s.order, hint)
}

// String renders the hints as lines starting with a newline, or "" if empty.
func (s *hintSet) String() string {
	var b strings.Builder
	for _, hint := range s.order {
		b.WriteString("\nhint: ")
		b.WriteString(hint)
	}
	return b.String()
}
