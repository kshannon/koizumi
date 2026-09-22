package outdated

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Trimmed real `brew outdated --json=v2` output.
const brewJSON = `{"formulae":[
 {"name":"abseil","installed_versions":["20260107.1"],"current_version":"20260817.0","pinned":false,"pinned_version":null},
 {"name":"tmux","installed_versions":["3.5a"],"current_version":"3.7b","pinned":true,"pinned_version":"3.5a"}],
 "casks":[
 {"name":"alt-tab","installed_versions":["11.4.3"],"current_version":"11.7.0","pinned":false,"pinned_version":null}]}`

func TestParseBrewOutdated(t *testing.T) {
	items, err := ParseBrewOutdated([]byte(brewJSON))
	if err != nil {
		t.Fatal(err)
	}
	want := []Item{
		{Source: "brew", Name: "abseil", Installed: "20260107.1", Latest: "20260817.0", Fix: "brew upgrade abseil"},
		{Source: "brew", Name: "tmux", Installed: "3.5a", Latest: "3.7b", Pinned: true, Fix: "pinned: brew unpin tmux, then brew upgrade tmux"},
		{Source: "cask", Name: "alt-tab", Installed: "11.4.3", Latest: "11.7.0", Fix: "brew upgrade --cask alt-tab"},
	}
	if len(items) != len(want) {
		t.Fatalf("got %d items, want %d: %+v", len(items), len(want), items)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Errorf("item %d:\n got  %+v\n want %+v", i, items[i], want[i])
		}
	}
}

func TestParseMasOutdated(t *testing.T) {
	// real mas pads names to a column width: the trailing spaces must not survive
	out := "1295203466   Microsoft Remote Desktop       (10.9.10 -> 10.9.11)\n497799835 Xcode (15.0 -> 15.1)\n"
	items := ParseMasOutdated(out)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2: %+v", len(items), items)
	}
	if items[0] != (Item{Source: "app-store", Name: "Microsoft Remote Desktop", Installed: "10.9.10", Latest: "10.9.11", Fix: "mas upgrade 1295203466"}) {
		t.Errorf("got %+v", items[0])
	}
	if ParseMasOutdated("") != nil {
		t.Error("empty output should give no items")
	}
}

func TestParseSoftwareUpdate(t *testing.T) {
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>LastSuccessfulDate</key><date>2026-09-22T03:31:01Z</date>
  <key>RecommendedUpdates</key><array>
    <dict><key>Display Name</key><string>macOS Tahoe 26.7</string><key>Display Version</key><string>26.7</string></dict>
    <dict><key>Display Name</key><string>Safari</string><key>Display Version</key><string>27.0</string></dict>
  </array>
</dict></plist>`
	items, checked, err := ParseSoftwareUpdate([]byte(plist), "26.5.2")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(checked.Format("2006-01-02"), "2026-09-22") {
		t.Errorf("last check = %v, want 2026-09-22", checked)
	}
	if len(items) != 2 || items[0].Name != "macOS Tahoe 26.7" || items[0].Installed != "26.5.2" || items[0].Latest != "26.7" {
		t.Fatalf("got %+v", items)
	}
	if items[0].Fix != "System Settings > General > Software Update" {
		t.Errorf("fix = %q", items[0].Fix)
	}
}

func TestProbeItemsAreNeverNullInJSON(t *testing.T) {
	for name, p := range map[string]Probe{
		"skipped": skipped(Probe{Source: "x"}, "why"),
		"error":   failed(Probe{Source: "x"}, "what", errTest),
		"ok":      done(Probe{Source: "x"}, nil),
	} {
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"items":[]`) {
			t.Errorf("%s probe: want \"items\":[] in %s", name, b)
		}
	}
}

var errTest = errors.New("boom")
