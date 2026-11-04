# registry logout

Remove the saved credentials for a registry.

The registry is a host with an optional port, as for `registry login`. The credentials for other registries are left alone. Logging out of a registry with no saved credentials is not an error.

No network call is made: this only edits the local credentials file.

## Examples

```
cargueiro registry logout registry.example.com
```
