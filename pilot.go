package main

// Reading and writing X-Wing Alliance pilot files (.plt).
//
// The file is a raw, packed PilotData struct with no header: its size is
// exactly sizeof(PilotData) = 152076 bytes. Only the fields this tool needs are
// modelled; everything else is preserved byte for byte by writing back a full
// copy of the original buffer with the few changed fields patched in place.
//
// Field offsets were taken from src/xwa/frontend/mission_setup.h in OpenXWA:
//
//   PilotData:
//     missionDirectoryId         46 - 4  = 42
//     missionDescriptionIds[7]   46
//     tourOfDutyMissions[255]    44282
//     campaignMode               151795
//     missionFileName            151807
//
//   PilotMission (48 bytes):
//     m00                        0
//     numberTimesFlown           4
//     firstPlaceCount            8
//     secondPlaceCount           12
//     thirdPlaceCount            16
//     completedCount             20
//     failedCount                24
//     bestScore                  28
//     bestTime                   32
//     bestPlacement              36
//     awardId                    40
//     bestBonus                  44

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

const (
	PilotDataSize = 152076

	pilotMissionSize = 48

	offMissionDirectoryID   = 42
	offMissionDescriptionID = 46
	offTourOfDutyMissions   = 44282
	offCampaignMode         = 151795
	offMissionFileName      = 151807

	pilotMissionOffTimesFlown = 4
	pilotMissionOffCompleted  = 20

	// The tour-of-duty directory. The game finds the current mission by taking
	// the first entry in MISSION.LST whose completedCount is 0, so that is the
	// directory index this tool writes.
	missionDirectoryTour = 4
)

// PilotMission is the subset of per-mission progress this tool reads and writes.
type PilotMission struct {
	TimesFlown int32
	Completed  int32
}

// Pilot is a loaded .plt file. Raw holds the original bytes; every write is
// applied to a copy of it so unrelated data is never disturbed.
type Pilot struct {
	Path string
	Raw  []byte

	Name                string
	MissionDirectoryID  int32
	MissionDescriptionID [7]int32
}

// LoadPilot reads and validates a .plt file.
func LoadPilot(path string) (*Pilot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) != PilotDataSize {
		return nil, fmt.Errorf("%s is %d bytes, expected %d: this does not look like a pilot file",
			path, len(raw), PilotDataSize)
	}

	p := &Pilot{Path: path, Raw: raw}
	p.Name = strings.TrimRight(string(raw[0:14]), "\x00")
	p.MissionDirectoryID = int32(binary.LittleEndian.Uint32(raw[offMissionDirectoryID:]))
	for i := 0; i < 7; i++ {
		p.MissionDescriptionID[i] = int32(binary.LittleEndian.Uint32(raw[offMissionDescriptionID+i*4:]))
	}
	return p, nil
}

// Mission returns the stored progress for the given mission index.
func (p *Pilot) Mission(idx int) PilotMission {
	base := offTourOfDutyMissions + idx*pilotMissionSize
	return PilotMission{
		TimesFlown: int32(binary.LittleEndian.Uint32(p.Raw[base+pilotMissionOffTimesFlown:])),
		Completed:  int32(binary.LittleEndian.Uint32(p.Raw[base+pilotMissionOffCompleted:])),
	}
}

// Patch returns a copy of the pilot bytes with the tour-of-duty progress set so
// that `target` is the next mission the game offers.
//
// The concourse picks the first MISSION.LST entry whose completedCount is 0, so
// every entry before `target` in the list must be complete and every entry from
// `target` onward must be cleared. Setting both directions makes the result
// independent of what the file held before.
//
// Mission indices come from MISSION.LST and are not contiguous, so the ordered
// list is passed in rather than assuming a numeric range. Indices that are not
// in the list are left untouched.
func (p *Pilot) Patch(ordered []MissionEntry, target int) ([]byte, error) {
	pos := -1
	for i, m := range ordered {
		if m.Index == target {
			pos = i
			break
		}
	}
	if pos < 0 {
		return nil, fmt.Errorf("mission index %d is not in the mission list", target)
	}

	out := make([]byte, len(p.Raw))
	copy(out, p.Raw)

	for i, m := range ordered {
		base := offTourOfDutyMissions + m.Index*pilotMissionSize
		if base+pilotMissionSize > offCampaignMode {
			return nil, fmt.Errorf("mission index %d is out of the pilot file's range", m.Index)
		}

		if i < pos {
			// Everything before the target must read as done.
			binary.LittleEndian.PutUint32(out[base+pilotMissionOffCompleted:], 1)
			if binary.LittleEndian.Uint32(out[base+pilotMissionOffTimesFlown:]) == 0 {
				binary.LittleEndian.PutUint32(out[base+pilotMissionOffTimesFlown:], 1)
			}
		} else {
			// The target and everything after it must read as not done.
			binary.LittleEndian.PutUint32(out[base+pilotMissionOffCompleted:], 0)
		}
	}

	// Point the tour directory at the target and make sure the pilot is in
	// campaign mode, or the concourse will offer a different mission.
	binary.LittleEndian.PutUint32(out[offMissionDirectoryID:], missionDirectoryTour)
	binary.LittleEndian.PutUint32(out[offMissionDescriptionID+missionDirectoryTour*4:], uint32(target))
	binary.LittleEndian.PutUint32(out[offCampaignMode:], 1)

	return out, nil
}

// Save writes data to path, keeping a single .bak copy of the previous file.
func Save(path string, data []byte) error {
	original, err := os.ReadFile(path)
	if err == nil {
		if err := os.WriteFile(path+".bak", original, 0o644); err != nil {
			return fmt.Errorf("backup failed, not writing: %w", err)
		}
	}
	return os.WriteFile(path, data, 0o644)
}
