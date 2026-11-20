package takeover

import (
	"errors"
	"fmt"

	"github.com/ebitengine/purego"
)

func originalBundlePath(path string) (string, error) {
	security, err := purego.Dlopen("/System/Library/Frameworks/Security.framework/Security", purego.RTLD_LAZY)
	if err != nil {
		return "", err
	}
	defer purego.Dlclose(security)
	foundation, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_LAZY)
	if err != nil {
		return "", err
	}
	defer purego.Dlclose(foundation)
	var isTranslocated func(uintptr, *bool, *uintptr) bool
	var originalURL func(uintptr, *uintptr) uintptr
	var createURL func(uintptr, []byte, int64, bool) uintptr
	var getPath func(uintptr, bool, []byte, int64) bool
	var release func(uintptr)
	for _, symbol := range []struct {
		library uintptr
		name    string
		target  any
	}{
		{security, "SecTranslocateIsTranslocatedURL", &isTranslocated},
		{security, "SecTranslocateCreateOriginalPathForURL", &originalURL},
		{foundation, "CFURLCreateFromFileSystemRepresentation", &createURL},
		{foundation, "CFURLGetFileSystemRepresentation", &getPath},
		{foundation, "CFRelease", &release},
	} {
		address, err := purego.Dlsym(symbol.library, symbol.name)
		if err != nil {
			return "", fmt.Errorf("resolve App Translocation: %w", err)
		}
		purego.RegisterFunc(symbol.target, address)
	}
	url := createURL(0, []byte(path), int64(len(path)), true)
	if url == 0 {
		return "", errors.New("cannot create app bundle URL")
	}
	defer release(url)
	var translocated bool
	var cfError uintptr
	ok := isTranslocated(url, &translocated, &cfError)
	if cfError != 0 {
		release(cfError)
	}
	if !ok || !translocated {
		return "", errors.New("cannot identify translocated app bundle")
	}
	cfError = 0
	original := originalURL(url, &cfError)
	if cfError != 0 {
		release(cfError)
	}
	if original == 0 {
		return "", errors.New("cannot resolve original app bundle; native messaging registration unchanged")
	}
	defer release(original)
	buffer := make([]byte, 4096)
	if !getPath(original, true, buffer, int64(len(buffer))) {
		return "", errors.New("cannot read original app bundle path")
	}
	for i, b := range buffer {
		if b == 0 {
			return string(buffer[:i]), nil
		}
	}
	return "", errors.New("unterminated original app bundle path")
}
