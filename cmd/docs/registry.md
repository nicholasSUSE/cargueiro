# registry

Manage the registry credentials cargueiro saves.

`cargueiro registry login` saves the credentials for a registry so that other commands, such as `push`, use them without being given a username and password every time. `cargueiro registry logout` removes them.

Credentials are saved per registry host and port, in a file of their own in your user config directory (`$CARGUEIRO_CONFIG_DIR` overrides the directory). Docker's and Helm's saved credentials are never read.

## Examples

```
# save credentials for a registry
echo "$PASSWORD" | cargueiro registry login registry.example.com --username admin --password-stdin
```

```
# forget them
cargueiro registry logout registry.example.com
```
