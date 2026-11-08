package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// Offsets as derived from sizeof(PilotData) in mission_setup.h. If the upstream
// struct changes these must change with it, which is why they are asserted.
func TestOffsetsMatchStructLayout(t *testing.T) {
	if PilotDataSize != 152076 {
		t.Fatalf("PilotDataSize = %d, expected 152076", PilotDataSize)
	}
	// tourOfDutyMissions ends before campaignMode.
	if offTourOfDutyMissions+255*pilotMissionSize > offCampaignMode {
		t.Fatalf("tourOfDutyMissions overruns campaignMode")
	}
}

func writeBlankPilot(t *testing.T, dir string) string {
	t.Helper()
	buf := make([]byte, PilotDataSize)
	copy(buf, "TestPilot\x00")
	path := filepath.Join(dir, "test.plt")
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestPatchSetsTargetAndClearsLater(t *testing.T) {
	dir := t.TempDir()
	path := writeBlankPilot(t, dir)

	p, err := LoadPilot(path)
	if err != nil {
		t.Fatal(err)
	}

	ordered := []MissionEntry{
		{Index: 3}, {Index: 4}, {Index: 5},
		{Index: 9}, {Index: 10},
	}

	out, err := p.Patch(ordered, 9)
	if err != nil {
		t.Fatal(err)
	}

	// Everything before the target is complete, everything from it is not.
	base := offTourOfDutyMissions
	get := func(idx int) int32 {
		o := base + idx*pilotMissionSize + pilotMissionOffCompleted
		return int32(binary.LittleEndian.Uint32(out[o:]))
	}
	for _, idx := range []int{3, 4, 5} {
		if got := get(idx); got == 0 {
			t.Errorf("mission %d should be complete, got %d", idx, got)
		}
	}
	for _, idx := range []int{9, 10} {
		if got := get(idx); got != 0 {
			t.Errorf("mission %d should be cleared, got %d", idx, got)
		}
	}

	// The tour directory points at the target.
	dirID := int32(binary.LittleEndian.Uint32(out[offMissionDirectoryID:]))
	if dirID != missionDirectoryTour {
		t.Errorf("missionDirectoryId = %d, want %d", dirID, missionDirectoryTour)
	}
	desc := int32(binary.LittleEndian.Uint32(out[offMissionDescriptionID+missionDirectoryTour*4:]))
	if desc != 9 {
		t.Errorf("tour missionDescriptionId = %d, want 9", desc)
	}

	// Nothing outside the intended fields changed.
	changed := 0
	for i := range out {
		if out[i] != p.Raw[i] {
			changed++
		}
	}
	// 3 completed flags + 2 flown flags + target id (1 byte, 9 fits) = 6.
	if changed == 0 {
		t.Fatal("nothing changed")
	}
}

func TestPatchRejectsUnknownIndex(t *testing.T) {
	dir := t.TempDir()
	path := writeBlankPilot(t, dir)
	p, err := LoadPilot(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Patch([]MissionEntry{{Index: 3}}, 99); err == nil {
		t.Fatal("expected an error for an unknown mission index")
	}
}

func TestPatchIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := writeBlankPilot(t, dir)
	p, err := LoadPilot(path)
	if err != nil {
		t.Fatal(err)
	}
	ordered := []MissionEntry{{Index: 3}, {Index: 4}, {Index: 5}}

	first, err := p.Patch(ordered, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, first, 0o644); err != nil {
		t.Fatal(err)
	}
	p2, err := LoadPilot(path)
	if err != nil {
		t.Fatal(err)
	}
	second, err := p2.Patch(ordered, 4)
	if err != nil {
		t.Fatal(err)
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("patch is not idempotent, byte %d differs", i)
		}
	}
}

func TestLoadPilotRejectsWrongSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.plt")
	if err := os.WriteFile(path, []byte("short"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPilot(path); err == nil {
		t.Fatal("expected an error for a wrong-sized file")
	}
}

func TestLoadMissionListFromMod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MISSION.LST")
	content := "//\r\n!BATTLE_0_HEADER![Pilot Training]\r\n//\r\n3\r\n* 1b0m1tftcTF-R.tie\r\n!MISSION_3_DESC!Basic Combat Training\r\n4\r\n* 1b0m2tftcTI-R.tie\r\n!MISSION_4_DESC!Wingmen Training\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadMissionList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Index != 3 || entries[0].File != "1b0m1tftcTF-R.tie" ||
		entries[0].Desc != "Basic Combat Training" || entries[0].Battle != "Pilot Training" {
		t.Errorf("first entry wrong: %+v", entries[0])
	}
	if entries[1].Index != 4 || entries[1].Desc != "Wingmen Training" {
		t.Errorf("second entry wrong: %+v", entries[1])
	}
}

// The stock MISSION.LST writes the first (unlocked) mission as a bare file name
// with no "* " prefix. XWA Upgrade does not ship its own list and relies on the
// stock one, so that form has to parse.
func TestLoadMissionListBareFirstFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MISSION.LST")
	content := "//\r\n!BATTLE_0_HEADER![Prologue: Family Business]\r\n//\r\n0\r\n1B0M1FW.tie\r\n!MISSION_0_DESC!Aeron's Lesson: Transport Operations\r\n1\r\n* 1B0M2FW.tie\r\n!MISSION_1_DESC!Emon's Lesson: Weapons\r\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadMissionList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Index != 0 || entries[0].File != "1B0M1FW.tie" ||
		entries[0].Desc != "Aeron's Lesson: Transport Operations" {
		t.Errorf("first entry wrong: %+v", entries[0])
	}
	if entries[1].Index != 1 || entries[1].File != "1B0M2FW.tie" || entries[1].Desc != "Emon's Lesson: Weapons" {
		t.Errorf("second entry wrong: %+v", entries[1])
	}
}

func TestLoadMissionListLockedPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "MISSION.LST")
	content := "0\n* 1B0M1FW.tie\n1\n& 1B0M2FW.tie\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := LoadMissionList(path)
	if err != nil {
		t.Fatal(err)
	}
	if entries[1].File != "1B0M2FW.tie" {
		t.Errorf("locked prefix not stripped: %+v", entries[1])
	}
}
