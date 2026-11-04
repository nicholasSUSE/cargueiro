# push

Push packaged Helm charts to an OCI registry.

Each argument is a chart archive (.tgz), such as the ones `cargueiro package` prints. A chart is pushed to `<registry>/<name>:<version>`, with name and version read from the chart inside the archive, not from the file name. A version with build metadata keeps its `+` on the command line (`110.0.3+up0.16.3`); the registry stores the tag with `_` (`110.0.3_up0.16.3`).

`--registry` is the destination, as `host[:port][/path]`. A leading `oci://`, `https://` or `http://` is ignored.

The pushed references are printed to stdout as `oci://<registry>/<name>:<tag>`, one per line, in argument order. Logs and errors go to stderr.

Every archive is attempted. If any fails, the others are still pushed, every error is reported at the end and the exit code is non-zero. When the failure has a known cause (missing or wrong credentials, a TLS or plain-HTTP mismatch, an untrusted certificate, an unreachable or unknown host), a `hint:` line after the errors says how to fix it. The same hint is shown once, however many archives hit it.

Authentication uses basic auth. Give the user with `--username` (or `CARGUEIRO_USERNAME`) and the password through `CARGUEIRO_PASSWORD` or, with `--password-stdin`, on standard input. There is no password flag, so the secret stays out of shell history and the process list. Without credentials the push is anonymous. Only the credentials given here are used; Docker's and Helm's stored credentials are never read.

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
