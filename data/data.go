// Package data holds the game data built into erpl: maps, missions, vehicles,
// weapons and their names. enlisted-gamedata writes the files from the installed game.
package data

import "embed"

//go:embed *.json maps
var FS embed.FS
