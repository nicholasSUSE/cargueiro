package oci

// CredentialFunc returns the username and the password to read from host, or
// empty strings to read anonymously. A nil CredentialFunc is anonymous.
type CredentialFunc func(host string) (username, password string, err error)

// FetchOptions returns the options to read from host.
func (c CredentialFunc) FetchOptions(host string, plainHTTP bool) (FetchOptions, error) {
	opts := FetchOptions{PlainHTTP: plainHTTP}
	if c == nil {
		return opts, nil
	}

	username, password, err := c(host)
	if err != nil {
		return FetchOptions{}, err
	}
	opts.Username, opts.Password = username, password
	return opts, nil
}
