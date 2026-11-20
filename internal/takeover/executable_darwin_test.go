package takeover

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExecutable(t *testing.T) {
	const translocated = "/private/var/folders/ab/AppTranslocation/uuid/d/jisu.app/Contents/MacOS/jisu"
	bundle := filepath.Join(t.TempDir(), "My jisu.app")
	stable := filepath.Join(bundle, "Contents", "MacOS", "jisu")
	if err := os.MkdirAll(filepath.Dir(stable), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stable, []byte("executable"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input, original, want string
		err                         error
	}{
		{"ordinary", stable, "", stable, nil},
		{"resolved", translocated, bundle, stable, nil},
		{"unavailable", translocated, "", "", errors.New("unavailable")},
		{"empty", translocated, "", "", nil},
		{"still translocated", translocated, filepath.Dir(filepath.Dir(filepath.Dir(translocated))), "", nil},
		{"missing original", translocated, "/missing/jisu.app", "", nil},
		{"not a bundle", "/private/var/folders/ab/AppTranslocation/uuid/jisu", bundle, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveExecutable(tc.input, func(path string) (string, error) {
				if path != "/private/var/folders/ab/AppTranslocation/uuid/d/jisu.app" {
					t.Fatalf("bundle = %q", path)
				}
				return tc.original, tc.err
			})
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("resolved = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestOriginalBundlePathUnavailable(t *testing.T) {
	if _, err := originalBundlePath(filepath.Join(t.TempDir(), "missing.app")); err == nil {
		t.Fatal("resolved a nonexistent translocated bundle")
	}
}
