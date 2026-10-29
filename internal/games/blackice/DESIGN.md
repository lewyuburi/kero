# BLACK ICE

A megacorporation's mainframe has fallen and its sector fills with malware. You
are a daemon, and the sector is yours to hold: five conduits of data run across
it, and what comes down them must not reach the core.

Defense by lanes, in the style of a one-bit handheld. Our own lore, names and
sprites; the mechanics are the genre's.

## The screen

320 by 240, as every game. Under the band of the score and the hint (16 pixels):
the **sector**, 9 columns by 5 rows of tiles of 28 pixels, 252 by 140, and under
it a row of five **cards** (48 by 28), near the pointer that plays in the sector.
Under them, the core, three chips, and the progress of the waves.

## Play

- **Watts** are what a program costs. The sector starts with 100. **Sparks** carry
  25: they fall in the sector now and then (every 9 seconds) and are given by
  dynamos; they are gone in 14 seconds if not taken.
- A **card** is a program with a cost and a time it takes to be ready again. Choose
  a card, then a tile: it is placed if the tile is free, the card ready and the
  watts enough.
- **Malware** comes in from the right along a conduit, and eats the first
  program in its way. One that gets to the left edge **breaches** the core and is
  gone: with the third breach the system is down.
- The sector is cleared when the last of the 8 waves is dead.

## Programs (the cards)

| Card   | Cost | Ready  | Does                                                    |
| ------ | ---- | ------ | ------------------------------------------------------- |
| PULSE  | 50   | 5 s    | Fires a bolt down its conduit every 1.4 s (2 damage)    |
| DYNAMO | 25   | 4 s    | Makes a spark every 9 s                                 |
| WALL   | 50   | 20 s   | Nothing; takes a lot to be eaten                        |
| MINE   | 25   | 30 s   | Arms in 4 s; blows up what steps on it and its neighbours |
| CRYO   | 75   | 5 s    | Fires a bolt that does 1 damage and slows for 3 s       |

## Malware

| Kind    | Health | Pace            | Eats      |
| ------- | ------ | --------------- | --------- |
| DRONE   | 20     | a tile in 4.7 s | 4 a second |
| WORM    | 12     | a tile in 2.7 s | 3 a second |
| TROJAN  | 56     | a tile in 5.3 s | 5 a second |

## Waves

The first at 20 seconds, then one every 25. Drones alone, then worms, then
trojans, in groups that grow; the last, the eighth, is all three kinds.

## Controls

The mouse: click a card, then a tile; click a spark to take it. The keyboard:
the arrows move over the sector, Enter changes the card, Space places it or takes
a spark. Escape goes back to the gallery.

## Time

A step is 1/30 of a second, always: the clock is the game's, not the frames'.
Everything the engine counts is in steps, so that a test can run a game in
no time.
