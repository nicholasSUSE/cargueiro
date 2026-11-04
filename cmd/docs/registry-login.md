# registry login

Save credentials for a registry.

The registry is a host with an optional port, such as `registry.example.com` or `localhost:5000`. An address with a path is refused, because credentials belong to a host. A leading `oci://`, `https://` or `http://` is ignored.

The username comes from `--username`, from `CARGUEIRO_USERNAME`, or is asked for on a terminal. The password comes from `--password`, standard input with `--password-stdin`, `CARGUEIRO_PASSWORD`, or a prompt that does not echo, in that order. `--password` is insecure: the password stays in your shell history and is visible in the process list, so a warning is printed. Prefer `--password-stdin` or the prompt.

Before anything is saved, the credentials are checked against the registry. If it rejects them, nothing is saved. If it accepts anonymous requests, the credentials cannot be checked and a warning says so.

The credentials are saved in a file of their own, mode 0600, in your user config directory (`$CARGUEIRO_CONFIG_DIR` overrides the directory). Without a Docker credential helper installed, the password is stored base64-encoded, which is not encryption, and a warning says so. Docker's and Helm's saved credentials are never read.

To reach a registry served over plain HTTP, such as a local one, use `--plain-http`. For a registry behind a private or self-signed CA, trust its certificate with `--ca-file`. The two cannot be combined. These only affect the check made at login.

## Examples

```
# the password from standard input
echo "$PASSWORD" | cargueiro registry login registry.example.com --username admin --password-stdin
```

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
