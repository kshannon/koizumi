package outdated

import "testing"

func TestSelfCompare(t *testing.T) {
	// go install ...@main stamps a pseudo-version ending in the 12-char commit sha
	inst := "v0.0.0-20260922070445-3f9f81691dfc"
	if behind, why := SelfBehind(inst, "3f9f81691dfc0a1b2c3d4e5f60718293a4b5c6d7"); behind || why != "" {
		t.Errorf("same commit must not be behind: %v %q", behind, why)
	}
	if behind, _ := SelfBehind(inst, "473f944aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); !behind {
		t.Error("a different commit on main means behind")
	}
	if behind, why := SelfBehind("(devel)", "473f944aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); behind || why == "" {
		t.Errorf("a source build cannot be compared: want not behind with a reason, got %v %q", behind, why)
	}
	if behind, _ := SelfBehind("v0.1.0", "473f944aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); behind {
		t.Error("a tagged release is compared by tag later, not by main's sha")
	}
}
