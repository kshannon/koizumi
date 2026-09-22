package apps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		facts    Facts
		override Updater
		want     Updater
	}{
		{"App Store receipt wins", Facts{MASReceipt: true, Cask: "x"}, "", AppStore},
		{"Sparkle feed means the app updates itself", Facts{SparkleFeed: true}, "", Self},
		{"bundled updater framework means self", Facts{UpdaterFramework: true}, "", Self},
		{"cask marked auto_updates means self", Facts{Cask: "raycast", CaskAutoUpdates: true}, "", Self},
		{"cask without any updater means brew", Facts{Cask: "letos"}, "", Brew},
		{"nothing known means unknown", Facts{}, "", Unknown},
		{"an override beats everything", Facts{MASReceipt: true}, Self, Self},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, why := Classify(tt.facts, tt.override)
			if got != tt.want {
				t.Fatalf("Classify(%+v) = %q, want %q", tt.facts, got, tt.want)
			}
			if why == "" {
				t.Fatalf("Classify(%+v) gave no reason", tt.facts)
			}
		})
	}
}

func TestScanReadsRealBundles(t *testing.T) {
	dir := t.TempDir()
	// A hand-installed app with a Sparkle feed in its Info.plist
	writeApp(t, dir, "Sparkly.app", `<key>CFBundleShortVersionString</key><string>1.2.3</string>
<key>SUFeedURL</key><string>https://example.com/appcast.xml</string>`)
	// An App Store app: receipt file, no feed
	writeApp(t, dir, "Store.app", `<key>CFBundleShortVersionString</key><string>9.0</string>`)
	if err := os.MkdirAll(filepath.Join(dir, "Store.app/Contents/_MASReceipt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Store.app/Contents/_MASReceipt/receipt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Scan(dir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		version string
		updater Updater
	}{
		"Sparkly": {"1.2.3", Self},
		"Store":   {"9.0", AppStore},
	}
	if len(got) != len(want) {
		t.Fatalf("Scan found %d apps, want %d: %+v", len(got), len(want), got)
	}
	for _, a := range got {
		w, ok := want[a.Name]
		if !ok {
			t.Fatalf("unexpected app %q", a.Name)
		}
		if a.Version != w.version || a.Updater != w.updater {
			t.Errorf("%s: got version %q updater %q, want %q %q", a.Name, a.Version, a.Updater, w.version, w.updater)
		}
	}
}

// writeApp creates a minimal .app bundle with an XML Info.plist holding the given keys.
func writeApp(t *testing.T, dir, name, keys string) {
	t.Helper()
	contents := filepath.Join(dir, name, "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
` + keys + `
</dict></plist>`
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFindsNestedAndBundleUpdaters(t *testing.T) {
	dir := t.TempDir()
	// Chrome-style: Keystone buried inside a versioned framework
	writeApp(t, dir, "Chromey.app", `<key>CFBundleShortVersionString</key><string>1</string>`)
	deep := filepath.Join(dir, "Chromey.app/Contents/Frameworks/Chromey Framework.framework/Versions/A/Frameworks/KeystoneRegistration.framework")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	// Firefox-style: an updater.app bundle next to the binary
	writeApp(t, dir, "Foxy.app", `<key>CFBundleShortVersionString</key><string>2</string>`)
	if err := os.MkdirAll(filepath.Join(dir, "Foxy.app/Contents/MacOS/updater.app/Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	// Google-style: a Keystone update URL in Info.plist, nothing else
	writeApp(t, dir, "Goog.app", `<key>KSUpdateURL</key><string>https://tools.google.com/service/update2</string>`)
	// Zoom-style: an updater helper app with a vendor-specific name
	writeApp(t, dir, "Zoomy.app", `<key>CFBundleShortVersionString</key><string>3</string>`)
	if err := os.MkdirAll(filepath.Join(dir, "Zoomy.app/Contents/Frameworks/ZoomyAutoUpdater.app/Contents"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Scan(dir, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("found %d apps, want 4", len(got))
	}
	for _, a := range got {
		if a.Updater != Self {
			t.Errorf("%s: got %q (%s), want self", a.Name, a.Updater, a.Reason)
		}
	}
}
