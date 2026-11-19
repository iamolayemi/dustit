package finder

import (
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/iamolayemi/dustit/internal/fsx"
	"github.com/iamolayemi/dustit/internal/sh"
)

// matchFiles walks roots for regular files accepted by keep, older than `days`.
func matchFiles(roots []string, days int, keep func(name string, size int64) bool, note string) []Item {
	cutoff := time.Now().AddDate(0, 0, -days)
	var out []Item
	walkUserFiles(DedupeRoots(roots), func(p string, st *syscall.Stat_t) {
		size := st.Blocks * 512
		if t := fsx.AddedOrEdited(st); t.Before(cutoff) && keep(filepath.Base(p), size) {
			out = append(out, Item{Path: p, Size: size, Age: t, Note: note + ", from " + humanAgo(t)})
		}
	})
	SortBySize(out)
	return out
}

var installerExt = map[string]bool{".dmg": true, ".pkg": true, ".mpkg": true, ".xip": true}

// Installers finds disk images and installer packages in Downloads and on the Desktop.
func Installers(days int) []Item {
	return matchFiles([]string{fsx.H("Downloads"), fsx.H("Desktop")}, days, func(name string, _ int64) bool {
		return installerExt[strings.ToLower(filepath.Ext(name))]
	}, "installer")
}

var screenshotName = regexp.MustCompile(`(?i)^(screenshot|screen shot|screen recording|capture d.écran|bildschirmfoto|captura de pantalla|schermafbeelding|istantanea|cleanshot)`)
var screenshotExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".heic": true, ".mov": true, ".mp4": true, ".tiff": true}

// ScreenshotFolders are the Desktop, Downloads, Pictures and the custom screenshot location.
func ScreenshotFolders() []string {
	roots := []string{fsx.H("Desktop"), fsx.H("Downloads"), fsx.H("Pictures")}
	if loc := sh.Line("defaults", "read", "com.apple.screencapture", "location"); loc != "" {
		roots = append(roots, fsx.Expand(loc))
	}
	return roots
}

// Screenshots finds screenshots and screen recordings older than `days`.
func Screenshots(days int) []Item {
	return matchFiles(ScreenshotFolders(), days, func(name string, _ int64) bool {
		return screenshotName.MatchString(name) && screenshotExt[strings.ToLower(filepath.Ext(name))]
	}, "screenshot")
}

var videoExt = map[string]bool{".mp4": true, ".mov": true, ".mkv": true, ".avi": true, ".m4v": true, ".webm": true, ".wmv": true, ".mts": true}

// LargeVideos finds video files of at least minB unchanged for `days`.
func LargeVideos(days int, minB int64) []Item {
	return matchFiles([]string{fsx.H("Movies"), fsx.H("Downloads"), fsx.H("Desktop"), fsx.H("Documents")}, days, func(name string, size int64) bool {
		return size >= minB && videoExt[strings.ToLower(filepath.Ext(name))]
	}, "video")
}
