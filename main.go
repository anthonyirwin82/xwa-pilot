// xwa-pilot sets which tour-of-duty mission an X-Wing Alliance pilot file is
// about to fly.
//
// Usage:
//
//	xwa-pilot mission <MISSION.LST> pilot <pilot.plt>
//
// Run with no arguments for help.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// modsDirExample returns an example mods directory for the host platform, so
// the help text matches where the user actually is.
func modsDirExample() string {
	switch runtime.GOOS {
	case "windows":
		return `%LOCALAPPDATA%\TotallyOpen\OpenXWA\mods`
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			home = "~"
		}
		return filepath.Join(home, "Library", "Application Support", "TotallyOpen", "OpenXWA", "mods")
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			home = "~"
		}
		return filepath.Join(home, ".local", "share", "TotallyOpen", "OpenXWA", "mods")
	}
}

// pilotDirExample returns an example pilot directory for the host platform.
func pilotDirExample() string {
	switch runtime.GOOS {
	case "windows":
		return `%LOCALAPPDATA%\TotallyOpen\OpenXWA`
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			home = "~"
		}
		return filepath.Join(home, "Library", "Application Support", "TotallyOpen", "OpenXWA")
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			home = "~"
		}
		return filepath.Join(home, ".local", "share", "TotallyOpen", "OpenXWA")
	}
}

func printHelp(w *os.File) {
	mods := modsDirExample()

	fmt.Fprintf(w, `xwa-pilot — set the next tour-of-duty mission in an X-Wing Alliance pilot file

USAGE
  xwa-pilot --mission <MISSION.LST> --pilot <pilot.plt>
  xwa-pilot -h | --help

ARGS
  --mission <path>   The MISSION.LST that lists the missions in order.
  --pilot <path>     The .plt pilot file to modify.

  Single-dash forms work too: -mission, -pilot.
  The two may be given in either order.

FLAGS
  -list            Print the missions and current progress, change nothing.
  -set <index>     Set the target mission without opening the UI.
  -yes             Skip the confirmation prompt when used with -set.

WHICH MISSION.LST
  TFTC Reimagined ships its own list:
    <mods>/tftc-reimagined-1.4.0/game/Missions/MISSION.LST

  XWA Upgrade does NOT ship a MISSION.LST. Use the base game's list instead:
    <game>/Missions/MISSION.LST
  The game resolves missions\mission.lst through the asset VFS, so XWA Upgrade
  missions are ordered and numbered by the stock file. Passing a mod path for
  XWA Upgrade will fail.

EXAMPLES
  Linux   (mods live in %s)

    TFTC Reimagined:
    xwa-pilot --mission ~/.local/share/TotallyOpen/OpenXWA/mods/tftc-reimagined-1.4.0/game/Missions/MISSION.LST \
              --pilot ~/.local/share/TotallyOpen/OpenXWA/AnthonyTFC0.plt

    XWA Upgrade (base game's list):
    xwa-pilot --mission ~/X-Wing/xwa-original/MISSIONS/MISSION.LST \
              --pilot ~/.local/share/TotallyOpen/OpenXWA/AnthonyXWA0.plt

  macOS   (mods live in ~/Library/Application Support/TotallyOpen/OpenXWA/mods)

    TFTC Reimagined:
    xwa-pilot --mission "$HOME/Library/Application Support/TotallyOpen/OpenXWA/mods/tftc-reimagined-1.4.0/game/Missions/MISSION.LST" \
              --pilot "$HOME/Library/Application Support/TotallyOpen/OpenXWA/AnthonyTFC0.plt"

    XWA Upgrade (base game's list):
    xwa-pilot --mission "/Applications/OpenXWA.app/Contents/Resources/Missions/MISSION.LST" \
              --pilot "$HOME/Library/Application Support/TotallyOpen/OpenXWA/AnthonyXWA0.plt"

  Windows (mods live in %%LOCALAPPDATA%%\TotallyOpen\OpenXWA\mods)

    TFTC Reimagined:
    xwa-pilot --mission "%%LOCALAPPDATA%%\TotallyOpen\OpenXWA\mods\tftc-reimagined-1.4.0\game\Missions\MISSION.LST" ^
              --pilot "%%LOCALAPPDATA%%\TotallyOpen\OpenXWA\AnthonyTFC0.plt"

    XWA Upgrade (base game's list):
    xwa-pilot --mission "C:\Program Files (x86)\Steam\steamapps\common\Star Wars X-Wing Alliance\Missions\MISSION.LST" ^
              --pilot "%%LOCALAPPDATA%%\TotallyOpen\OpenXWA\AnthonyXWA0.plt"

KEYS (in the list)
  up/down, k/j     move          enter   choose the highlighted mission
  pgup/pgdn        page          y/n     confirm or cancel
  g/G              first/last    q/esc   quit

NOTES
  The game must be closed. It rewrites the pilot file on exit and would
  overwrite the change.

  A backup is written to <pilot.plt>.bak before any change.

  The game offers the first mission in MISSION.LST whose completed count is 0,
  so this tool marks every earlier mission complete.

  Only tour-of-duty progress is changed. Skirmish progress is left alone.

  Use --list first to check the list matches the mod you are about to play.
`,
		mods)
}

func main() {
	// -h/--help before anything else, so they work without arguments.
	for _, a := range os.Args[1:] {
		switch a {
		case "-h", "--help", "help":
			printHelp(os.Stdout)
			return
		}
	}

	if len(os.Args) == 1 {
		printHelp(os.Stderr)
		os.Exit(2)
	}

	var (
		missionsPath = flag.String("mission", "", "path to the mod's MISSION.LST")
		pilotPath    = flag.String("pilot", "", "path to the pilot .plt file")
		listOnly     = flag.Bool("list", false, "print missions and current progress, change nothing")
		setIdx       = flag.Int("set", -1, "set the target mission index without the UI")
		assumeYes    = flag.Bool("yes", false, "do not ask for confirmation with -set")
	)

	flag.Usage = func() { printHelp(os.Stderr) }
	flag.Parse()

	if *missionsPath == "" || *pilotPath == "" {
		fmt.Fprintf(os.Stderr, "error: both --mission and --pilot are required\n\n")
		printHelp(os.Stderr)
		os.Exit(2)
	}

	pilot, err := LoadPilot(*pilotPath)
	if err != nil {
		fail(err)
	}
	missions, err := LoadMissionList(*missionsPath)
	if err != nil {
		fail(err)
	}

	switch {
	case *listOnly:
		printList(pilot, missions, *missionsPath)
	case *setIdx >= 0:
		runSet(pilot, missions, *setIdx, *assumeYes)
	default:
		runUI(pilot, missions, *missionsPath)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

// printList writes a plain-text report: every mission, its current state, and
// which one the game will offer next.
func printList(pilot *Pilot, missions []MissionEntry, listPath string) {
	fmt.Printf("pilot:   %s\n", pilot.Path)
	fmt.Printf("name:    %s\n", pilot.Name)
	fmt.Printf("list:    %s\n", listPath)
	fmt.Println()

	next := -1
	for _, m := range missions {
		if pilot.Mission(m.Index).Completed == 0 {
			next = m.Index
			break
		}
	}

	fmt.Printf("%-6s %-28s %-42s %s\n", "INDEX", "BATTLE", "MISSION", "STATUS")
	prevBattle := "\x00"
	for _, m := range missions {
		b := m.Battle
		if b == prevBattle {
			b = ""
		}
		prevBattle = m.Battle

		pm := pilot.Mission(m.Index)
		status := "locked"
		switch {
		case m.Index == next:
			status = "next"
		case pm.Completed > 0:
			status = "done"
		}

		fmt.Printf("%-6d %-28s %-42s %s\n", m.Index, trimTo(b, 28), trimTo(m.Desc, 42), status)
	}

	fmt.Println()
	if next < 0 {
		fmt.Println("next mission: none (all missions complete)")
		return
	}
	for _, m := range missions {
		if m.Index == next {
			fmt.Printf("next mission: %d - %s (%s)\n", m.Index, m.Desc, m.File)
			break
		}
	}
}

// runSet applies the change without the UI.
func runSet(pilot *Pilot, missions []MissionEntry, target int, assumeYes bool) {
	var targetEntry *MissionEntry
	for i := range missions {
		if missions[i].Index == target {
			targetEntry = &missions[i]
			break
		}
	}
	if targetEntry == nil {
		fail(fmt.Errorf("mission index %d is not in the list", target))
	}

	if !assumeYes {
		fmt.Printf("Set pilot %q to mission %d - %s?\n", pilot.Name, targetEntry.Index, targetEntry.Desc)
		fmt.Printf("This rewrites %s (a .bak backup is kept). [y/N] ", pilot.Path)
		var answer string
		fmt.Scanln(&answer)
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("aborted")
			return
		}
	}

	patched, err := pilot.Patch(missions, target)
	if err != nil {
		fail(err)
	}
	if err := Save(pilot.Path, patched); err != nil {
		fail(err)
	}
	fmt.Printf("set to mission %d - %s\n", targetEntry.Index, targetEntry.Desc)
}

func runUI(pilot *Pilot, missions []MissionEntry, listPath string) {
	m := newModel(pilot, missions, listPath)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fail(err)
	}
}
