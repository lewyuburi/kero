# This fork

Kero (fork) is Kero with changes of its own, kept for its owner's use. It
installs as its own app (`sh.kero.fork`), takes no updates, and shares Kero's
data and `kerod`. This page is how it is kept.

## Branches

| Branch      | What it is |
| ----------- | ---------- |
| `main`      | egoist/kero's `main`, as it is. Only fast-forwards. |
| `fork/main` | `main` with the fork's changes over it: what is built and installed. |

The remotes: `origin` is egoist/kero, `lew` is this fork's.

## What the fork adds

Each is a commit or a few, in files of their own where it can be, so that what
Kero changes under them is settled once, in a merge.

- **Layout:** `card-gap` and `pane-gap` (and the corners that follow them),
  `tab-layout` with the tab bar across the top (`tabs.go`), ⌥⌘B, and sessions in
  no project (`loose-sessions`, with `new-session-loose` to start them from the +,
  New Session and the shortcut), middle-click closing tabs (`middle-click-close`)
  and dragging tabs and projects into order (`tab-reorder`, `reorder.go`).
- **Keys and hooks:** `keybind` sequences and key modes (`chords.go`), `pane.resize.*`;
  `on-status` hooks; `kero api` (kerod's methods, described and called).
- **Browser:** Kero's own (snapshot, refs, cookies), with `press`, `scroll`,
  `url` and `title`, and **Agent** in the header while a command is at the page.
- **Status rings** painted at 30 frames a second, not at every refresh.
- **Flags** (`internal/settings/flags.go`, Settings › Advanced): `horizontal-tabs`,
  `custom-spacing`, `loose-sessions`, `middle-click-close` and `tab-reorder` start
  disabled, as experiments do, so that the fork looks like Kero until one is enabled.
- **`third_party/mygo`:** MyGo with `Element.MiddleClicked` and `MiddlePressed`
  (`ui/input.go`), and `Window.SetTrafficLightPosition`, so the macOS
  window buttons follow the layout with the window open (`moveLights`). The change
  is `third_party/mygo-traffic-light-position.patch`; see `third_party/README.md`.
- **`mygo.json`:** the name, the identifier and no updates.

## Keeping up with Kero

```sh
git fetch origin
git switch main && git merge --ff-only origin/main
git switch fork/main && git merge main
```

A conflict is settled in that merge, once: `rerere` is on, so git remembers how.
Then, before anything is installed:

```sh
go vet ./... && go tool mygo vet .
go test ./...
GOOS=linux go build ./... && GOOS=windows go build ./...
```

Two tests fail where `KERO_*` is set or ssh is missing, on Kero's own `main` too:
`kero/internal/cli` `TestSplitHost` and `kero/internal/remote` `TestBridgeScript`.

If Kero moves `mygo`, `third_party/mygo` is made again from the new version:
copy the module, apply the patch (`git apply --directory=third_party/mygo`), and
prune what Kero does not build (docs, website, examples, packages, scripts, tests).
Once MyGo has the method itself, drop the `replace` in `go.mod`, the folder and
`moveLights`.

`mygo.json`'s `version` follows Kero's. Tag what is installed, as `fork-2.1.24`.

To keep merges small: put what is new in a file of its own, change Kero's files
little and by a line or two, and add a test for what is added, so that a merge
that breaks it says so.

## Build and install

```sh
go tool mygo build -sign - -skip-notarize -platform darwin/arm64
cp -R "build/darwin-arm64/Kero (fork).app" /Applications/
```

`kerod` outlives the app: after a build that changes it, restart it in Settings ›
Advanced, which ends the programs the sessions run (they come back as after a
reboot).
