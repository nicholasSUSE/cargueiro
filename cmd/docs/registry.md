# registry

Manage the registry login cargueiro saves.

`cargueiro registry login` saves the credentials for one registry, and how to reach it, so that other commands such as `push` use them without being told every time. `cargueiro registry logout` forgets them. Only one login is kept: log out before logging in to another registry.

The login is a file of its own in your user config directory (`$CARGUEIRO_CONFIG_DIR` overrides the directory). Docker's and Helm's saved credentials are never read.

Target-Audience: Chart Owners, Release Team and Automation

## Examples

```
# save credentials for a registry
echo "$PASSWORD" | cargueiro registry login registry.example.com --username admin --password-stdin
```

```
# forget them
cargueiro registry logout
```
