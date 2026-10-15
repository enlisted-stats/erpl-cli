package ui

import (
	"fmt"
	"strings"
	"time"
)

// Clock formats a time since the start of the recording as m:ss, or h:mm:ss from an hour.
func Clock(d time.Duration) string {
	s := int(d / time.Second)
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// Model is the vehicle model of a template, the part before the first "+".
func Model(template string) string {
	m, _, _ := strings.Cut(template, "+")
	return m
}

// Date formats a moment in local time; the zero time is "-".
func Date(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}
