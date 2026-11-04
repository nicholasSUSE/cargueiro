# registry logout

Forget the saved login.

This removes everything `registry login` saved, including the plain HTTP or CA setting, so `push` goes back to HTTPS with the system trust store and no credentials. You can then log in to another registry.

It takes no arguments and makes no network call. Logging out when nothing is saved is not an error, and it also removes a login file that has become unreadable.

## Examples

```
cargueiro registry logout
```
