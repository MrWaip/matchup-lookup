package main

import (
	"database/sql"
	"fmt"
)

var spellNames = map[int64]string{
	1: "Cleanse", 3: "Exhaust", 4: "Flash", 6: "Ghost", 7: "Heal",
	11: "Smite", 12: "Teleport", 13: "Clarity", 14: "Ignite",
	21: "Barrier", 32: "Mark",
}

var runeNames = map[int64]string{
	8005: "Press the Attack", 8008: "Lethal Tempo", 8010: "Conqueror", 8021: "Fleet Footwork",
	9101: "Absorb Life", 9111: "Triumph", 8009: "Presence of Mind",
	9104: "Legend: Alacrity", 9105: "Legend: Tenacity", 9103: "Legend: Bloodline", 8014: "Coup de Grace", 8017: "Cut Down", 8299: "Last Stand",
	8112: "Electrocute", 8128: "Dark Harvest", 9923: "Hail of Blades",
	8126: "Cheap Shot", 8139: "Taste of Blood", 8143: "Sudden Impact",
	8136: "Zombie Ward", 8120: "Ghost Poro", 8138: "Eyeball Collection",
	8135: "Treasure Hunter", 8105: "Relentless Hunter", 8106: "Ultimate Hunter", 8134: "Ingenious Hunter",
	8214: "Summon Aery", 8229: "Arcane Comet", 8230: "Phase Rush",
	8224: "Nullifying Orb", 8226: "Manaflow Band", 8275: "Nimbus Cloak",
	8210: "Transcendence", 8234: "Celerity", 8233: "Absolute Focus",
	8237: "Scorch", 8232: "Waterwalking", 8236: "Gathering Storm",
	8437: "Grasp of the Undying", 8439: "Aftershock", 8465: "Guardian",
	8446: "Demolish", 8463: "Font of Life", 8401: "Shield Bash",
	8429: "Conditioning", 8444: "Second Wind", 8473: "Bone Plating",
	8451: "Overgrowth", 8453: "Revitalize", 8242: "Unflinching",
	8351: "Glacial Augment", 8360: "Unsealed Spellbook", 8369: "First Strike",
	8306: "Hextech Flashtraption", 8304: "Magical Footwear", 8321: "Cash Back", 8313: "Triple Tonic",
	8345: "Biscuit Delivery", 8347: "Cosmic Insight", 8410: "Approach Velocity", 8352: "Time Warp Tonic", 8316: "Jack of All Trades",
}

func spellName(id sql.NullInt64) string { return loadoutName(id, spellNames) }
func runeName(id sql.NullInt64) string  { return loadoutName(id, runeNames) }

func loadoutName(id sql.NullInt64, names map[int64]string) string {
	if !id.Valid || id.Int64 == 0 {
		return "?"
	}
	if name, ok := names[id.Int64]; ok {
		return name
	}
	return fmt.Sprintf("#%d", id.Int64)
}

func loadoutPair(name func(sql.NullInt64) string, first, second sql.NullInt64) string {
	return name(first) + " + " + name(second)
}
