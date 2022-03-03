package js

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/diamondburned/gotkit/app"
	"github.com/dop251/goja"
	"github.com/pkg/errors"
)

// CompileFromURL downloads and compiles a JavaScript file at the given URL into
// a goja.Program. JS files are cached persistently on the disk.
func CompileFromURL(ctx context.Context, uri string) (*goja.Program, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, errors.Wrap(err, "invalid URL")
	}

	name := path.Base(u.Path)

	switch u.Scheme {
	case "file":
		b, err := os.ReadFile(u.Path)
		if err != nil {
			return nil, errors.Wrap(err, "file error")
		}
		return goja.Compile(name, string(b), false)

	case "http", "https":
		s, err := download(ctx, u.String(), filepath.Join(assetDir(ctx), name))
		if err != nil {
			return nil, errors.Wrap(err, "http error")
		}
		return goja.Compile(name, s, false)

	default:
		return nil, fmt.Errorf("unknown scheme %q", u.Scheme)
	}
}

func download(ctx context.Context, url, dst string) (string, error) {
	b, err := os.ReadFile(dst)
	if err == nil {
		return string(b), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", errors.Wrap(err, "cannot make request")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "cannot send request")
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}
	defer resp.Body.Close()

	dir := filepath.Dir(dst)

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return "", errors.Wrap(err, "cannot make cache directory")
	}

	var str strings.Builder
	r := io.TeeReader(resp.Body, &str)

	f, err := os.CreateTemp(filepath.Dir(dst), ".download.*")
	if err != nil {
		return "", errors.Wrap(err, "cannot make temp download file")
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", errors.Wrap(err, "cannot download")
	}

	if err := f.Close(); err != nil {
		return "", errors.Wrap(err, "cannot close download file")
	}

	if err := os.Rename(f.Name(), dst); err != nil {
		return "", errors.Wrap(err, "cannot commit downloaded file")
	}

	return str.String(), nil
}

// assetDir gets the assets directory. It panics if the directory cannot be
// obtained.
func assetDir(ctx context.Context) string {
	app := app.FromContext(ctx)
	if app != nil {
		return app.CachePath("assets", "js")
	}
	return filepath.Join(os.TempDir(), "jotup", "assets", "js")
}
