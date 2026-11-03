package crawler

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/maksqi/site-sucker/internal/config"
)

func localPath(cfg config.Config, start, target *url.URL, kind ResourceKind) string {
	root := filepath.Clean(cfg.Destination)
	if cfg.Output.CreateHostFolder {
		root = filepath.Join(root, sanitizeSegment(start.Hostname()))
	}
	if !strings.EqualFold(start.Hostname(), target.Hostname()) {
		root = filepath.Join(root, "_external", sanitizeSegment(target.Scheme+"_"+target.Host))
	} else if !sameOrigin(start, target) {
		root = filepath.Join(root, "_origin", sanitizeSegment(target.Scheme+"_"+target.Host))
	}

	cleanPath := path.Clean("/" + target.EscapedPath())
	segments := strings.Split(strings.TrimPrefix(cleanPath, "/"), "/")
	cleanSegments := make([]string, 0, len(segments)+1)
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			continue
		}
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			decoded = segment
		}
		cleanSegments = append(cleanSegments, sanitizeSegment(decoded))
	}
	if len(cleanSegments) == 0 {
		cleanSegments = append(cleanSegments, "index.html")
	} else if kind == KindDocument && (strings.HasSuffix(target.Path, "/") || path.Ext(target.Path) == "") {
		cleanSegments = append(cleanSegments, "index.html")
	}

	if cfg.Scope.KeepQuery && target.RawQuery != "" {
		hash := sha256.Sum256([]byte(target.RawQuery))
		last := cleanSegments[len(cleanSegments)-1]
		ext := filepath.Ext(last)
		base := strings.TrimSuffix(last, ext)
		cleanSegments[len(cleanSegments)-1] = fmt.Sprintf("%s_q_%x%s", base, hash[:4], ext)
	}
	return filepath.Join(append([]string{root}, cleanSegments...)...)
}

func sanitizeSegment(value string) string {
	if value == "" {
		return "_"
	}
	var encoded strings.Builder
	const hexadecimal = "0123456789ABCDEF"
	for _, current := range []byte(value) {
		if (current >= 'a' && current <= 'z') || (current >= '0' && current <= '9') || strings.ContainsRune("._-()", rune(current)) {
			encoded.WriteByte(current)
			continue
		}
		encoded.WriteByte('%')
		encoded.WriteByte(hexadecimal[current>>4])
		encoded.WriteByte(hexadecimal[current&0x0f])
	}
	result := encoded.String()
	base := strings.TrimSuffix(result, filepath.Ext(result))
	switch base {
	case ".", "..", "con", "prn", "aux", "nul", "com1", "com2", "com3", "com4", "com5", "com6", "com7", "com8", "com9", "lpt1", "lpt2", "lpt3", "lpt4", "lpt5", "lpt6", "lpt7", "lpt8", "lpt9":
		result = "%00" + result
	}
	if strings.HasSuffix(result, ".") {
		result = strings.TrimSuffix(result, ".") + "%2E"
	}
	if len(result) > 180 {
		hash := sha256.Sum256([]byte(value))
		result = result[:160] + fmt.Sprintf("_%x", hash[:8])
	}
	return result
}

func writeFile(cfg config.Config, filename string, body []byte, remoteTime time.Time) (bool, error) {
	if info, err := os.Stat(filename); err == nil {
		switch cfg.Output.Replacement {
		case config.ReplaceSkip:
			return false, nil
		case config.ReplaceNewer:
			if !remoteTime.IsZero() && !remoteTime.After(info.ModTime()) {
				return false, nil
			}
			if remoteTime.IsZero() {
				existing, readErr := os.ReadFile(filename)
				if readErr == nil && bytes.Equal(existing, body) {
					return false, nil
				}
			}
		}
	} else if !os.IsNotExist(err) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return false, err
	}
	temporary, err := os.CreateTemp(filepath.Dir(filename), ".site-sucker-part-*")
	if err != nil {
		return false, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return false, err
	}
	if _, err := temporary.Write(body); err != nil {
		temporary.Close()
		return false, err
	}
	if err := temporary.Close(); err != nil {
		return false, err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(filename)
	}
	if err := os.Rename(temporaryName, filename); err != nil {
		return false, err
	}
	if cfg.Output.PreserveTimes && !remoteTime.IsZero() {
		_ = os.Chtimes(filename, remoteTime, remoteTime)
	}
	return true, nil
}
