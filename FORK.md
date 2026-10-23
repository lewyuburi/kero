# This fork

Kero (fork) is Kero with a few changes of its own. It installs as its own app
(`sh.kero.fork`) and shares Kero's data and `kerod`. This page is how it is kept,
so that what is worth sending back can be, cleanly.

## Branches

| Branch               | What it is |
| -------------------- | ---------- |
| `main`               | egoist/kero's `main`, as it is. Only fast-forwards. |
| `pr/card-gap`        | `card-gap` and `pane-gap`, and the corners that follow them. |
| `pr/horizontal-tabs` | `tab-layout`, the tab bar with a capsule a project, and ⇧⌘B. On `pr/card-gap`. |
| `pr/loose-sessions`  | `loose-sessions`: a session in no project. On `main`, alone. |
| `pr/ring-frame-rate` | The status rings painted at 30 frames a second, not at every refresh. On `main`, alone. |
| `pr/browser-snapshot` | `kero browser snapshot`, refs (`@e3`) for elements, `press`, `scroll`, `url` and `title`. On `main`, alone. |
| `pr/browser-agent-badge` | The browser's header says **Agent** while a command is at the page. On `main`, alone. |
| `pr/browser-web-only` | `kero browser open` and `--url` take web pages alone. On `main`, alone. |
| `fork/main`          | What is built and installed: the branches above merged, then the fork's own layer. |

The `pr/*` branches are what could be sent to egoist/kero: plain settings that
leave Kero as it is until chosen, no flags, no fork name, no vendored code, a
commit a change in the style of their log, with tests, both catalogs and the docs. They
are never pushed from here unless that is the plan.

The fork's layer, on top of them in `fork/main`, in this order:

1. `third_party/mygo`: MyGo with `Window.SetTrafficLightPosition`, so the macOS
   window buttons follow the layout with the window open. MyGo has no such method;
   `third_party/mygo-traffic-light-position.patch` is the change as a pull request
   to MyGo would be, on every backend with its tests, and goes first.
2. `moveLights`: uses it as the gap or the tabs change.
3. Flags (`internal/settings/flags.go`, Settings › Advanced): `horizontal-tabs`,
   `custom-spacing` and `loose-sessions` start disabled, as experiments do.
4. `mygo.json`: the name, the identifier and no updates.

## Keeping up with Kero

```sh
git fetch origin
git switch main && git merge --ff-only origin/main
git switch fork/main && git merge main
```

Kero's own browser, panes and hosts are the fork's: nothing of theirs is kept
apart. The fork's changes are commits on `fork/main` over `main`; a conflict is
settled there, once, as it comes.

The `pr/*` branches that are not `pr/browser-*` were last rebased on Kero 2.0.7.
Their changes are on `fork/main` over 2.1.9, as the commits of the same names, with
what 2.1.x changed settled in them (a few tests, the hosts model, the side panel). One
that is to go upstream is made again on `main` from its commits, then rebased as
the others were.

A change to a `pr/*` branch is made there, then merged into `fork/main`; only the
fork's layer is changed on `fork/main` itself.

## Build and install

```sh
go tool mygo build -sign - -skip-notarize -platform darwin/arm64
cp -R "build/darwin-arm64/Kero (fork).app" /Applications/
```

`kerod` outlives the app: after a build that changes it, restart it in Settings ›
Advanced, which ends the programs the sessions run (they come back as after a
reboot).

## Before anything goes upstream

- Open an issue first, with the feature request template; these are changes to
  how Kero looks and groups things, which its author may want another way.
- One change a branch, rebased on their `main`, `go vet`, `go tool mygo vet .`,
  `go test ./...`, and `GOOS=linux`/`GOOS=windows` builds.
- English and Simplified Chinese strings, `docs/` and the key's reference line.
- Commit messages in their style: an imperative sentence, then what changed, file
  by file where that helps. No trailers.
- MyGo's own change goes to egoist/mygo first, with its end-to-end test, and Kero
  then bumps MyGo; until it is released, `card-gap` moves the macOS window buttons
  when Kero next opens instead of at once.
