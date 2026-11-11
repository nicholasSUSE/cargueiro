# Releasing a chart with cargueiro

For chart owners. This is how a chart gets from your repository to the staging registry, and from there to the `staging` branch of `rancher/charts`. The steps run in the GitHub Actions workflow of your repository. Each one is a cargueiro command that exits with 1 and says why on stderr when something is wrong, so a step that fails stops the workflow and nothing after it runs.

Install cargueiro in the workflow with `curl -fsSL https://github.com/rancher/cargueiro/releases/latest/download/install.sh | sh -`. It checks the download against the checksums of the release, and `CARGUEIRO_VERSION` names a release to pin.

## The whole path

```mermaid
flowchart TD
    start(["<b>Release a version</b><br/>of your chart"])

    subgraph owner["Your repository · GitHub Actions"]
        stamp["<b>cargueiro stamp</b><br/>the final version and<br/>helm.sh/images"]
        check["<b>cargueiro check chart</b><br/>annotations · images · version"]
        package["<b>cargueiro package</b><br/>check, build the .tgz,<br/>compare every file"]
        push["<b>cargueiro push</b><br/>to the staging registry,<br/>a version is pushed once"]
        stamp --> check --> package --> push
    end

    failed(["<b>The workflow stops</b><br/>fix the chart and run it again"])
    registry[("<b>Staging registry</b><br/>stgregistry.suse.com<br/>rancher/catalog/my-chart")]
    trigger{{"<b>Start the workflow</b><br/>in rancher/charts"}}

    subgraph charts["rancher/charts · GitHub Actions"]
        ship["<b>cargueiro ship</b><br/>the entry and the logo"]
        index["<b>cargueiro index</b><br/>pull the archive,<br/>build index.yaml"]
        ship --> index
    end

    staging(["<b>staging branch of rancher/charts</b><br/>entries · assets · index.yaml<br/>how it is merged is still open"])

    start --> stamp
    owner -. "a step fails" .-> failed
    push --> registry --> trigger --> ship
    index --> staging

    classDef cmd fill:#e8f1ff,stroke:#3b6fd4,stroke-width:1.5px,color:#0b2a5b
    classDef store fill:#fff4d6,stroke:#c98a00,stroke-width:1.5px,color:#5a3d00
    classDef stop fill:#fde8e8,stroke:#d43b3b,stroke-width:1.5px,color:#6b0f0f
    classDef todo fill:#f1f1f1,stroke:#7a7a7a,stroke-width:1.5px,stroke-dasharray:5 4,color:#333
    classDef begin fill:#e9f7ec,stroke:#2f9e44,stroke-width:1.5px,color:#0b3d1a
    class stamp,check,package,push,ship,index cmd
    class registry store
    class failed stop
    class trigger,staging todo
    class start begin
```

## Before you start

- The chart is listed in `rules.yaml`, which is compiled into cargueiro. It says what each chart must be: its kind, its family and the annotations it needs. A chart that is not listed is refused by every command; ask the release team to add it.
- The images in `values.yaml` have a `repository` and a `tag`, and the tag is the one you are releasing. A tag of `latest`, or none, is refused. Set `appVersion` and the image tags to the release first, as `stamp` changes neither.
- `Chart.yaml` has the `catalog.cattle.io/rancher-version` annotation. The Rancher version in it decides the major of the chart version.

## 1. Stamp the chart

```
cargueiro stamp charts/my-chart --version 112.0.0+up0.13.0
```

It puts the final version on the chart and lists its images in `helm.sh/images`. Only the `version` line and that annotation of `Chart.yaml` change, so the diff in your workflow shows nothing else. It refuses a version that does not follow the rules, and an image whose tag is `latest`.

`--version` is yours: it is the version of your release, and `stamp` does not compare it with anything but the rules. Give it. Without it, `stamp` works the version out from the versions already in the staging registry, which reads the registry: give `--username` and the password the way `push` takes them if it asks for credentials. `--dry-run` prints what would change and writes nothing. Charts for Rancher 2.15 and before are not supported.

## 2. Check the chart

```
cargueiro check chart charts/my-chart
```

It reads `Chart.yaml` and `values.yaml`, and nothing else in the folder, and checks them against the rules of the chart. The report has three sections:

- `annotations`: the annotations the chart needs for its kind, and their form.
- `hip15-annotations`: `helm.sh/images` lists every image in `values.yaml` ([HIP-0015](https://helm.sh/community/hips/hip-0015/)).
- `version`: the version is `1XY.Z.W+upA.B.C`, and `1XY` matches the Rancher version of the chart (`111` for 2.16, `112` for 2.17).

It goes after `stamp` because a chart that is not stamped yet fails in `version` and in `hip15-annotations`, which is what `stamp` puts right. The `annotations` section is the one `stamp` cannot fix, and where you will usually see an error. `--output json` prints the report as JSON.

## 3. Package the chart

```
cargueiro package charts/my-chart --output-dir dist
```

It makes the archive (`.tgz`) and prints its path, and nothing else, on stdout. It does not change the chart: stamp it first. Before packaging, it runs the same check as `check chart` on the folder, so a chart that is not valid is not packaged, and after packaging it checks the archive. It then compares every file of the folder with the same file in the archive. If the archive is not valid, or differs from the folder, it is removed and the step fails.

## 4. Push the chart

```
cargueiro push dist/my-chart-112.0.0+up0.13.0.tgz --registry stgregistry.suse.com/rancher/catalog
```

The registry is the staging registry of the chart's edition. The username and the password come from `CARGUEIRO_USERNAME` and `CARGUEIRO_PASSWORD`, which your workflow reads from its secrets. It prints the reference it pushed. On SUSE registries it enforces two rules, and refuses with the reason otherwise:

- A chart goes only to the repository of its edition, never to the one for images.
- It never replaces a version that is there: a tag is pushed once. Running the step again for the same version fails, on purpose.

## 5. Ship it to rancher/charts

After the push, the chart is in the staging registry, but not in `rancher/charts`. Your workflow tells `rancher/charts` that a chart was staged, with a `repository_dispatch` of type `chart-staged` that has two fields in its `client_payload`: `chart`, the name, and `version`, as in its `Chart.yaml` with the `+`. Nothing else is read from the event, because `rancher/charts` takes the rest from the registry. There it runs `cargueiro ship` for that version and then `cargueiro index`, which write the entry, the logo, the archive and the index to the `staging` branch. A version that was shipped already fails there, on purpose.

```
gh api "repos/rancher/charts/dispatches" \
  -f event_type=chart-staged \
  -f "client_payload[chart]=my-chart" \
  -f "client_payload[version]=112.0.0+up0.13.0"
```

A shared action that does steps 1 to 5 in your workflow is planned (see rancher/cargueiro#54). Until it exists, you send the event yourself, as above.

## What your repository needs

The event is sent with a token that can write to `rancher/charts`. These are the things to have, and to ask the infrastructure team (EIO) for what is missing. The answers to the open questions in rancher/cargueiro#54 will be written here.

- A GitHub App installed on `rancher/charts` with the repository permission Contents: write, and its `appId` and `privateKey` in Vault under your repository (`github/app-credentials`). `rancher/webhook` is the example to follow.
- The staging registry credentials in Vault under your repository, for `push` (`rancher-prime-stg-registry/credentials`).
- Your workflow job has the `id-token: write` permission, so it can log in to Vault.

## All together

```
cargueiro stamp charts/my-chart --version "$CHART_VERSION"
cargueiro check chart charts/my-chart
archive=$(cargueiro package charts/my-chart --output-dir dist)
cargueiro push "$archive" --registry stgregistry.suse.com/rancher/catalog
gh api "repos/rancher/charts/dispatches" -f event_type=chart-staged \
  -f "client_payload[chart]=my-chart" -f "client_payload[version]=$CHART_VERSION"
```
