
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="1088" height="256" viewBox="56 408 1088 256"><g/><g/><g/><g/><g/><g/><g/><g id="div1"><g><rect width="1088" height="256" x="56" y="408" fill="rgb(10, 14, 24)" stroke="rgb(27, 35, 53)" stroke-width="1px" rx="24" ry="24"/></g><g id="svg1" mask="url(#mask-for-svg11)"><g/><g/><g/><g/><g/><g/><g/><mask id="mask-for-svg11"><rect width="160" height="160" x="184" y="456" fill="#ffffff"/></mask><g transform="matrix(1.6, 0, 0, 1.6, 166.4, 417.6)"><defs alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="rgb(0, 0, 0)" stroke="none"><filter id="svg1-glowC" x="-30%" y="-30%" width="160%" height="160%"><feGaussianBlur stdDeviation="1.2" result="b"/><feMerge><feMergeNode in="b"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs><g fill="none" stroke-linejoin="round" stroke-linecap="round" filter="url(&quot;#svg1-glowC&quot;)" alignment-baseline="baseline" color="rgb(230, 237, 247)" stroke="none"><path d="M50 70 L16 84 M50 70 L106 92 M50 42 L50 70" stroke="rgb(34, 211, 238)" stroke-width="1" stroke-opacity="0.45" stroke-dasharray="2px, 2px" alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M23 58.75 V86.75 M30 61.5 V89.5 M37 64.25 V92.25 M44 67 V95 M51 69.75 V97.75 M58 72.5 V100.5 M65 75.25 V103.25" stroke="rgb(34, 211, 238)" stroke-width="0.8px" stroke-opacity="0.3" alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M24.5 69.2 L48.3 59.4 L63.9 65.6 L40.1 75.4 Z M24.5 83.2 L40.1 89.4 L40.1 75.4 L24.5 69.2 Z M40.1 89.4 L63.9 79.6 L63.9 65.6 L40.1 75.4 Z" stroke="rgb(163, 230, 53)" stroke-width="1.5px" alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M43.5 72.5 L67.3 62.7 L83 68.9 L59.2 78.7 Z M43.5 90.7 L59.2 96.9 L59.2 78.7 L43.5 72.5 Z M59.2 96.9 L83 87.1 L83 68.9 L59.2 78.7 Z" stroke="rgb(250, 204, 21)" stroke-width="1.5px" alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M64.2 84.9 L84.6 76.5 L95.8 80.9 L75.4 89.3 Z M64.2 97.5 L75.4 101.9 L75.4 89.3 L64.2 84.9 Z M75.4 101.9 L95.8 93.5 L95.8 80.9 L75.4 89.3 Z" stroke="rgb(251, 113, 133)" stroke-width="1.5px" alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M30.1 58.9 L47.1 51.9 L58.3 56.3 L41.3 63.3 Z M30.1 68.7 L41.3 73.1 L41.3 63.3 L30.1 58.9 Z M41.3 73.1 L58.3 66.1 L58.3 56.3 L41.3 63.3 Z" stroke="rgb(232, 121, 249)" stroke-width="1.5px" alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M16 56 L50 42 L106 64 L72 78 Z M16 56 L16 84 L72 106 L106 92 L106 64 M72 78 L72 106" stroke="rgb(34, 211, 238)" stroke-width="2px" alignment-baseline="baseline" color="rgb(230, 237, 247)" fill="none" stroke-linecap="round" stroke-linejoin="round"/></g></g></g><g id="div2"><g id="div3"><text color="rgba(0, 0, 0, 0)" dominant-baseline="text-after-edge" font-family="Space Grotesk, system-ui, sans-serif" font-size="100px" font-size-adjust="none" font-stretch="100%" font-style="normal" font-variant="normal" font-weight="700" direction="ltr" letter-spacing="-2px" text-decoration="none" text-anchor="start" text-rendering="auto" unicode-bidi="isolate" word-spacing="0px" writing-mode="horizontal-tb" user-select="text" fill="rgba(0, 0, 0, 0)"><tspan xml:space="preserve" x="380" y="581.5" textLength="444.8999938964844" lengthAdjust="spacingAndGlyphs">cargueiro</tspan></text></g><g id="div4"><text color="rgb(167, 179, 198)" dominant-baseline="text-after-edge" font-family="JetBrains Mono, monospace" font-size="20px" font-size-adjust="none" font-stretch="100%" font-style="normal" font-variant="normal" font-weight="400" direction="ltr" letter-spacing="normal" text-decoration="none" text-anchor="start" text-rendering="auto" unicode-bidi="isolate" word-spacing="0px" writing-mode="horizontal-tb" user-select="text" fill="rgb(167, 179, 198)"><tspan xml:space="preserve" x="380" y="604" textLength="636" lengthAdjust="spacingAndGlyphs">package · push · validate · list — helm charts on OCI</tspan></text></g></g></g></svg>

# cargueiro

Small, single-purpose tools to package Helm charts and push them to OCI registries.

## Build

```
make build
./bin/cargueiro --help
```

## Commands

Each command is documented in its own file under `cmd/docs/`. That file is the source of truth for `cargueiro <command> --help` and for this table.

<!-- COMMANDS:START -->
| Command | Description |
|---------|-------------|
| [`package`](cmd/docs/package.md) | Package a Helm chart into a .tgz archive. |
| [`push`](cmd/docs/push.md) | Push a packaged Helm chart to an OCI registry. |
| [`registry`](cmd/docs/registry.md) | Manage the registry login cargueiro saves. |
| [`registry login`](cmd/docs/registry-login.md) | Save credentials and connection settings for a registry. |
| [`registry logout`](cmd/docs/registry-logout.md) | Forget the saved login. |
| [`version`](cmd/docs/version.md) | Print the cargueiro version. |
<!-- COMMANDS:END -->

## Development

See [docs/DEVELOPING.md](docs/DEVELOPING.md).

Contributions made with LLM agents follow [docs/AGENTS.md](docs/AGENTS.md).

LLM Policy notice: All contributions need a human reviewer responsible for it.
