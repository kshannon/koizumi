// Package when renders times as "3h ago".
package when

import (
	"fmt"
	"time"
)

// Ago is Since(t, now).
func Ago(t time.Time) string { return Since(t, time.Now()) }

// Since renders how long before now t was: "just now", "3h ago", "5d ago".
func Since(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		return "just now"
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
