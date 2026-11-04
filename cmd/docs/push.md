# push

Push packaged Helm charts to an OCI registry.

Each argument is a chart archive (.tgz), such as the ones `cargueiro package` prints. A chart is pushed to `<registry>/<name>:<version>`, with name and version read from the chart inside the archive, not from the file name. A version with build metadata keeps its `+` on the command line (`110.0.3+up0.16.3`); the registry stores the tag with `_` (`110.0.3_up0.16.3`).

`--registry` is the destination, as `host[:port][/path]`. A leading `oci://`, `https://` or `http://` is ignored.

The pushed references are printed to stdout as `oci://<registry>/<name>:<tag>`, one per line, in argument order. Logs and errors go to stderr.

Every archive is attempted. If any fails, the others are still pushed, every error is reported at the end and the exit code is non-zero. When the failure has a known cause (missing or wrong credentials, a TLS or plain-HTTP mismatch, an untrusted certificate, an unreachable or unknown host), a `hint:` line after the errors says how to fix it. The same hint is shown once, however many archives hit it.

Pushing to `registry.suse.com/rancher/catalog` or `stgregistry.suse.com/rancher/catalog`, or anywhere under them, is refused: charts published there must never be overwritten. The check looks only at the address (host names are compared case-insensitively, and the port is ignored), runs before any password is read or any connection is made, and has no flag to turn it off. Any other registry, local ones included, may be overwritten: pushing a chart that already exists replaces it.

Authentication uses basic auth. The credentials come from the first of these that applies:

1. `--username` (or `CARGUEIRO_USERNAME`) with the password from `CARGUEIRO_PASSWORD` or, with `--password-stdin`, from standard input. There is no password flag, so the secret stays out of shell history and the process list.
2. The credentials saved by `cargueiro registry login` for exactly this host and port. They are only used when no username or password is given, and never for another host.
3. None: the push is anonymous.

If the saved credentials are rejected, a hint says to log in again. Only cargueiro's own saved credentials are read: Docker's and Helm's are never touched.

Connections use HTTPS and the system trust store. For a registry behind a private or self-signed CA, trust its certificate with `--ca-file`. `--plain-http` talks plain HTTP instead, for local testing; it cannot be combined with `--ca-file`.

## Examples

```
# push packaged charts, password from the environment
export CARGUEIRO_PASSWORD=...
cargueiro push assets/fleet-crd-110.0.3+up0.16.3.tgz --registry registry.example.com/charts --username admin
```

```
# package and push in one pipeline
cargueiro package charts/fleet-crd/110.0.3+up0.16.3 | xargs -d '\n' cargueiro push --registry registry.example.com/charts --username admin
```

```
# registry with a self-signed certificate, password from stdin
echo "$PASSWORD" | cargueiro push chart.tgz --registry localhost:5051 --username admin --password-stdin --ca-file ca.crt
```

```
# plain HTTP, local testing only
cargueiro push chart.tgz --registry localhost:5050 --username admin --plain-http
```
