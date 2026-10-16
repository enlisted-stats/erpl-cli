<h1 align="center">erpl-cli</h1>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.26+-00add8?style=flat-square" alt="Go 1.26+" />
  <img src="https://img.shields.io/badge/platform-windows-0078d6?style=flat-square" alt="Windows" />
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="MIT License" /></a>
</p>

<p align="center">
  A tool for taking a close look at <a href="https://enlisted.net">Enlisted</a> replays.
</p>

It shows players, kills, weapons, vehicles, structures and capture points right in the terminal, and builds
an HTML report with an interactive battle map: where the infantry fought, where squads spawned, where the
vehicles drove and where they were knocked out.

![ERPL](docs/replays.png)

## Installation

1. Download the archive from [releases](../../releases) and unpack it into any folder.
2. Download replays from [enlisted.net/replays](https://enlisted.net/replays) and put them into the `replays` folder.
3. Open a terminal in that folder and run `erpl list`.

The archive has everything you need:

```
erpl.exe   the program, with the maps and names from the game built in
replays/   put your replays here
reports/   reports are saved here
```

erpl reads replays of one game version, the one it was built for. `erpl list` marks replays of other
versions in grey. `erpl info` shows what it can about them, the other commands do not open them.

## Which replay opens

Every replay has a number. Numbers follow the order in which the files appeared in the folder: the oldest
replay is number 1, and a freshly downloaded one gets the highest number. Without a number, a command opens
the last downloaded replay. To open another one, pass its number from `erpl list`.

```
erpl players
erpl players 3
```

## Commands

The commands print in English and show the names of missions, vehicles and weapons as the game has them
in English.

### list

The replays in the `replays` folder, newest first. If there are many replays, the list is split into pages.
Pick a page with the `-p` flag.

```
erpl list
erpl list -p 2
```

### info

The mission, mode and campaign, the winner, the date, the length of the battle, the number of players in
each team and the game version.

```
erpl info
erpl info 3
```

### players

Players by team: kills, headshots, deaths, tanks and planes destroyed, assists, captures, structures built
and score. The title of each team shows its army and whether it won. Players from the same platoon share a
letter, and bots are labeled.

```
erpl players
erpl players 3
```

### kills

The kill log: who killed whom, when and with what. Destroyed vehicles are in the log too. The notes say
when a kill was a headshot, was made from a vehicle or was a team kill.

- `--player` shows only kills where the killer's or the victim's name contains the given text.
- `--weapon` shows only kills by a weapon whose name contains the given text, for example `ppsh`.
- `--kind` shows only kills by one kind of weapon, for example `rifle`, `grenade`, `cannon` or `artillery`.
- `--vehicles` shows only destroyed vehicles.

```
erpl kills
erpl kills 3
erpl kills 3 --player kaban
erpl kills 3 --kind artillery
erpl kills 3 --vehicles
```

### vehicles

Every vehicle in the battle: its kind, who called it in and when, who knocked it out and with what, how long
it stayed in the battle, how many soldiers sat in it and how far it drove. Guns built by engineers are listed
too, with the kind `gun`.

```
erpl vehicles
erpl vehicles 3
```

### weapons

What each team killed with: kills of enemy soldiers and vehicles by weapon, most kills first, and how many
of them were headshots. Team kills and suicides are not counted.

- `-k`, `--by-kind` counts kills by kind of weapon instead of by weapon.

```
erpl weapons
erpl weapons 3
erpl weapons 3 -k
```

### structures

What engineers built. By default structures are grouped by name: how many each team built, how many were
destroyed and how long they stood on average.

- `-a`, `--all` lists every structure separately: who built it and when, where it stands and when it was destroyed.
- `-k`, `--kind` shows only structures whose name or id contains the given text, for example `rally`.

```
erpl structures
erpl structures 3
erpl structures 3 -a
erpl structures 3 -a -k rally
```

### zones

Capture points: when each one was active and which army held it.

```
erpl zones
erpl zones 3
```

### report

Builds an HTML report about the battle, saves it to the `reports` folder and opens it in the browser.

- `-l`, `--lang` sets the language of the report: `en` or `ru`. The default is `en`.
- `--no-open` only saves the report without opening it.

```
erpl report
erpl report 3
erpl report 3 -l ru
erpl report 3 -l ru --no-open
```

The report is saved as a single file of about a megabyte. It opens without the internet, and you can send
it to a friend.

## The report

The report is one interactive map of the battle. The header shows the mission, the mode and the winner.

### Layers

On the right you turn on what the map shows. The team filter above the layers applies to all of them.

- **Infantry.** A heatmap of where soldiers spent the most time.
- **Spawn points.** Infantry and vehicle bases, rally points and APCs. The number on an icon shows how many
  squads spawned there. A base that closed before the end of the chosen time is pale.
- **Vehicles.** Routes of tanks, APCs, planes, bikes and guns built by engineers, and where they were
  knocked out. The kinds can be turned off one by one.
- **Deaths.** Where soldiers died. The deaths can be filtered by weapon: small arms, explosives, vehicles,
  planes and artillery.
- **Knockdowns.** Where soldiers were knocked down and whether they were revived.
- **Structures.** Fortifications, guns and boxes built by engineers.

### The card

Click anything on the map or in the lists to open its card. The card of a player shows their stats, and the
map draws lines from where they shot to everyone they killed. The card of a vehicle shows its route, who
knocked it out and with what. The card of a spawn point shows who spawned there and how many times.

Under the layers there are lists of players, vehicles and kills. They follow the chosen team and time.

### Stages of the battle

The timeline is at the bottom of the report. The report splits the battle into stages the same way the
game does: each stage is tied to its own capture points. Click a stage to see only that part. You can also
drag over the timeline to pick any stretch of the battle.

For the chosen stretch, the map shows only what happened during that time, along with the capture points,
bases and battle area borders that were active then. The bars on the timeline show how many kills each team
made.

## Game data

The maps and the names of missions, modes, armies, vehicles, weapons, capture points and structures come
from the game files. They are in the `data` folder and are built into `erpl.exe`.

After a game patch, the data is made again with
[enlisted-gamedata](https://github.com/enlisted-stats/enlisted-gamedata) from the installed game, and
erpl-cli is built again:

```
enlisted-gamedata --dir "path/to/Enlisted" --out data --lang ru,en
go build ./cmd/erpl
```

A test checks that the data is of the same game version as the replays erpl reads.

## Building from source

You need Go 1.26 or newer.

```
go build ./cmd/erpl
```

Replays are read by the [erpl](https://github.com/enlisted-stats/erpl) library.

## Credits

- Interface icons: [Lucide](https://lucide.dev), ISC License.
- Map icons: [game-icons.net](https://game-icons.net) by Lorc, Delapouite and other authors,
  [CC BY 3.0](https://creativecommons.org/licenses/by/3.0/).

## License

The code is released under the [MIT License](LICENSE). The maps and names from the game in the `data` folder
are not covered by it.

## Legal

This is an unofficial project, not affiliated with Gaijin. The maps, names and other game materials in the
`data` folder belong to their respective owners.
