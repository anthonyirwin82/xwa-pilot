package main

// Parsing MISSION.LST.
//
// The format is a flat sequence of lines. A mission entry is three lines: the
// mission index, the file (prefixed with "* "), and the description. Header
// lines start with "!" and "//" acts as both a comment and a separator.
//
// Example:
//
//	//
//	!BATTLE_0_HEADER![Pilot Training]
//	//
//	3
//	* 1b0m1tftcTF-R.tie
//	!MISSION_3_DESC!Basic Combat Training
//	4
//	* 1b0m2tftcTI-R.tie
//	!MISSION_4_DESC!Wingmen Training
//
// The loader (src/xwa/frontend/mission_setup.c) takes the index with atoi, the
// file from column 2 of the "* " line, and the description as the remainder of
// the line after the first "!...!" token. Lines are CRLF in the shipped file,
// so they are trimmed before use.
//
// The list order matters: the game offers the first entry whose completedCount
// is 0, which is why the tool works from this order rather than the indices.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// MissionEntry is one mission in MISSION.LST.
type MissionEntry struct {
	Index   int    // mission index, as used to index tourOfDutyMissions[]
	File    string // mission file name, e.g. 1b0m1tftcTF-R.tie
	Desc    string // description as written in the list
	Battle  string // enclosing !BATTLE_*_HEADER! section name
}

// LoadMissionList parses a MISSION.LST file.
func LoadMissionList(path string) ([]MissionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	var (
		entries   []MissionEntry
		battle    string
		pendingID int = -1
	)

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || line == "//" {
			continue
		}

		// Section header, e.g. !BATTLE_0_HEADER![Pilot Training]
		if strings.HasPrefix(line, "!BATTLE_") {
			if open := strings.Index(line, "["); open >= 0 {
				if close := strings.Index(line[open:], "]"); close > 0 {
					battle = line[open+1 : open+close]
				}
			}
			continue
		}

		// Other directive lines, e.g. the description line.
		if strings.HasPrefix(line, "!") {
			if pendingID >= 0 && len(entries) > 0 && entries[len(entries)-1].Index == pendingID {
				if parts := strings.SplitN(line, "!", 3); len(parts) == 3 {
					entries[len(entries)-1].Desc = strings.TrimSpace(parts[2])
					pendingID = -1
				}
			}
			continue
		}

		// Mission file line. Three forms, as handled by the loader
		// (mission_setup.c):
		//   "* <file>"  normal
		//   "& <file>"  forced locked
		//   "<file>"    the first entry in the stock list has no prefix
		if strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "& ") || looksLikeMissionFile(line) {
			if len(entries) > 0 {
				file := strings.TrimSpace(line)
				if strings.HasPrefix(file, "*") || strings.HasPrefix(file, "&") {
					file = strings.TrimSpace(file[1:])
				}
				entries[len(entries)-1].File = file
			}
			continue
		}

		// Otherwise it should be a mission index.
		if id, err := strconv.Atoi(line); err == nil {
			entries = append(entries, MissionEntry{Index: id, Battle: battle})
			pendingID = id
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("%s: no missions found", path)
	}
	for _, e := range entries {
		if e.File == "" {
			return nil, fmt.Errorf("%s: mission %d has no file", path, e.Index)
		}
	}
	return entries, nil
}

// looksLikeMissionFile reports whether a line is a bare mission file name with
// no "* " prefix. The stock MISSION.LST writes the first (unlocked) mission that
// way. The check is deliberately narrow so it cannot swallow an index or a
// directive: a mission file ends in .tie/.skm and contains no spaces.
func looksLikeMissionFile(line string) bool {
	if strings.ContainsAny(line, " \t") {
		return false
	}
	lower := strings.ToLower(line)
	return strings.HasSuffix(lower, ".tie") || strings.HasSuffix(lower, ".skm")
}
