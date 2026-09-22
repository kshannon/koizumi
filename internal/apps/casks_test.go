package apps

import "testing"

// A trimmed, real `brew info --cask --json=v2 --installed` payload: one self-updating cask, one not.
const caskJSON = `{"casks":[
 {"token":"raycast","installed":"1.103.10","version":"2.4.1.0","auto_updates":true,
  "artifacts":[{"app":["Raycast.app"]},{"zap":[{"trash":["~/Library/Caches/com.raycast.macos"]}]}]},
 {"token":"letos","installed":"4.0.1","version":"4.0.3","auto_updates":null,
  "artifacts":[{"app":["Letos.app"]}]}
]}`

func TestParseCasksKeysByAppBundle(t *testing.T) {
	got, err := ParseCasks([]byte(caskJSON))
	if err != nil {
		t.Fatal(err)
	}
	r, ok := got["Raycast.app"]
	if !ok || r.Token != "raycast" || !r.AutoUpdates {
		t.Fatalf("Raycast.app = %+v, ok=%v; want token raycast, auto_updates true", r, ok)
	}
	l, ok := got["Letos.app"]
	if !ok || l.Token != "letos" || l.AutoUpdates {
		t.Fatalf("Letos.app = %+v, ok=%v; want token letos, auto_updates false", l, ok)
	}
}
