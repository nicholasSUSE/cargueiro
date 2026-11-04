# registry login

Save credentials and connection settings for a registry.

The registry is a host with an optional port, such as `registry.example.com` or `localhost:5000`. An address with a path is refused. If a login is already saved, `login` stops and asks you to run `registry logout` first.

The username comes from `--username`, `CARGUEIRO_USERNAME`, or a prompt. The password comes from `--password`, `--password-stdin`, `CARGUEIRO_PASSWORD`, or a prompt that does not echo, in that order. `--password` is insecure, because it stays in your shell history and the process list, and it warns.

The credentials are checked against the registry before anything is saved, and a rejection saves nothing. A registry that accepts anonymous requests cannot check them, so a warning is printed and the login is saved anyway.

`--plain-http` reaches the registry over HTTP, for local testing. `--ca-file` trusts the CA certificates in a PEM file, for a private or self-signed CA. They cannot be combined. Both are used for the check and then remembered for this registry. The contents of the CA file are saved, so moving the file later changes nothing.

Everything is saved in `login.json`, mode 0600, in your user config directory (`$CARGUEIRO_CONFIG_DIR` overrides the directory). The password is stored there in plain text, and a warning says so.

## Examples

```
# asked for the username and password on a terminal
cargueiro registry login registry.example.com
```

```
# a local registry over plain HTTP
echo "$PASSWORD" | cargueiro registry login localhost:5050 --username admin --password-stdin --plain-http
```

```
# a registry with a self-signed certificate
echo "$PASSWORD" | cargueiro registry login localhost:5051 --username admin --password-stdin --ca-file ca.crt
```
