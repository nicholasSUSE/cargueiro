# push

Push a packaged Helm chart to an OCI registry.

The argument is one chart archive (.tgz), such as the one `cargueiro package` prints. A chart is pushed to `<registry>/<name>:<version>`, with name and version read from the chart inside the archive, not from the file name. A `+` in the version becomes `_` in the tag: `110.0.3+up0.16.3` is stored as `110.0.3_up0.16.3`.

`--registry` is the destination, as `host[:port][/path]`. The pushed reference is printed to stdout as `oci://<registry>/<name>:<tag>`; errors go to stderr and the exit code is non-zero.

For credentials, give `--username` (or `CARGUEIRO_USERNAME`) and the password through `CARGUEIRO_PASSWORD` or `--password-stdin`. Without them, the login saved by `cargueiro registry login` is used when it is for this registry; otherwise the push is anonymous.

The connection is HTTPS with the system trust store, unless the login saved plain HTTP or a CA certificate for this registry. Those are set at login only.

On the SUSE registries named in `rules.yaml`, what may be pushed depends on the chart. A chart listed in `rules.yaml` goes only to the chart repository of its edition. Anything else is an image dependency and goes only to the image path, never to a chart repository. Everything else is refused, before any password is read or any connection is made. A chart or image that already exists there is never overwritten: if the registry has the tag, or cannot say, nothing is pushed. The host name is compared without regard to case and the port is ignored. On any other registry nothing is restricted, and pushing a chart that already exists replaces it.

## Examples

```
# push a packaged chart, password from the environment
export CARGUEIRO_PASSWORD=...
cargueiro push assets/fleet-crd-110.0.3+up0.16.3.tgz --registry registry.example.com/charts --username admin
```

```
# after logging in once, push needs no credentials and no flags
cargueiro push chart.tgz --registry localhost:5050/charts
```
