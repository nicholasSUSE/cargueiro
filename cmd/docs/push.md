# push

Push a packaged Helm chart to an OCI registry.

The argument is one chart archive (.tgz), such as the one `cargueiro package` prints. A chart is pushed to `<registry>/<name>:<version>`, with name and version read from the chart inside the archive, not from the file name. A `+` in the version becomes `_` in the tag: `110.0.3+up0.16.3` is stored as `110.0.3_up0.16.3`.

`--registry` is the destination, as `host[:port][/path]`. The pushed reference is printed to stdout as `oci://<registry>/<name>:<tag>`; errors go to stderr and the exit code is non-zero.

For credentials, give `--username` (or `CARGUEIRO_USERNAME`) and the password through `CARGUEIRO_PASSWORD`. Without them, the login saved by `cargueiro registry login` is used when it is for this registry; otherwise the push is anonymous.

The connection is HTTPS with the system trust store, unless the login saved plain HTTP or a CA certificate for this registry. Those are set at login only.

On the SUSE registries named in `rules.yaml`, what may be pushed depends on the chart. A chart listed in `rules.yaml` goes only to the chart repository of its edition. Anything else is an image dependency and goes only to the image path, never to a chart repository. Everything else is refused, before any password is read or any connection is made. A chart or image that already exists there is never overwritten: if the registry has the tag, or cannot say, nothing is pushed. The host name is compared without regard to case and the port is ignored. On any other registry nothing is restricted, and pushing a chart that already exists replaces it.

With `--family`, the arguments are the archives of every chart of one family in `rules.yaml`, such as `fleet`, `fleet-agent` and `fleet-crd`, and nothing else: a chart that is missing, given twice or from another family is an error, and the charts must all have the same version. The name of each chart is read from its archive, and the archives can be given in any order. Before any secret is read or the network is used, every chart is checked against the rules, and if one is refused none is pushed. The charts are then pushed one after the other in the order of the family, the main chart first.

On the SUSE registries, the registry is asked about every chart before any is pushed, and a chart it has already is skipped and never replaced, so that running the command again finishes what an earlier run left. A chart is skipped only if what the registry holds is the archive given, which is checked by comparing the digest of its chart layer with the sha256 of the archive; if the content is other, the command fails before any chart is pushed and says which charts differ, because a version is never overwritten. On any other registry nothing is asked, and a chart that is there is replaced, as for one chart. An upload that fails because of the network, a timeout, or an answer that says the registry is busy (429) or broken (5xx) is tried again, up to 5 attempts, waiting 30 seconds after the first failure and twice as long after each one that follows. An answer that refuses the request (400, 401, 403, 404), a certificate that is not trusted and a host that does not exist are not tried again. If an attempt fails and the registry has the chart afterwards, the chart counts as pushed. When a chart fails for good, the charts after it are not tried, the report on stderr says what happened to every chart, and the exit code is 1. The references of the charts are printed on stdout, one on each line in the order of the family, only when all of them are in the registry.

Target-Audience: Chart Owners and Automation

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

```
# push every chart of a family
cargueiro push --family fleet dist/fleet-110.0.3+up0.16.3.tgz dist/fleet-agent-110.0.3+up0.16.3.tgz dist/fleet-crd-110.0.3+up0.16.3.tgz --registry stgregistry.suse.com/rancher/catalog
```
