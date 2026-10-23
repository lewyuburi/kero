# third_party

## mygo

A copy of [mygo](https://github.com/egoist/mygo) at the version `go.mod` names
(v0.6.3-0.20261022115212-d89af4a66c5e), without its docs, website, examples,
packages and tests, which Kero does not build. `go.mod` points at it with a
`replace`.

It differs from upstream by one method, `Window.SetTrafficLightPosition(x, y)`,
which moves the macOS window buttons of a window made with
`WindowOptions.TrafficLightPosition` after it is open, as the option does as it
is made. It is on every backend, as mygo asks: macOS moves them, the others do
nothing, the fake records where they went. `mygo-traffic-light-position.patch`
is the whole change against mygo, with its tests and docs, as a pull request
to mygo would be.

Kero calls it as the layout changes (`moveLights`, platform.go). To go back to
upstream once mygo has it, drop the `replace` from `go.mod`, delete this folder
and `moveLights`; the buttons then take their place as the window opens.
