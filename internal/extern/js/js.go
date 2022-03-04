package js

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/diamondburned/gotkit/app"
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/require"
	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
)

var nodeRegistry = new(require.Registry)

// CompileFromURL downloads and compiles a JavaScript file at the given URL into
// a goja.Program. JS files are cached persistently on the disk.
func LoadFromURL(ctx context.Context, rt *goja.Runtime, uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return errors.Wrap(err, "invalid URL")
	}

	name := path.Base(u.Path)

	switch u.Scheme {
	case "file":
		b, err := os.ReadFile(u.Path)
		if err != nil {
			return errors.Wrap(err, "file error")
		}
		p, err := goja.Compile(name, string(b), false)
		if err != nil {
			return errors.Wrap(err, "compile error")
		}
		if _, err := rt.RunProgram(p); err != nil {
			return errors.Wrap(err, "cannot execute compiled file")
		}

	case "http", "https":
		s, err := download(ctx, u.String(), filepath.Join(assetDir(ctx), name))
		if err != nil {
			return errors.Wrap(err, "http error")
		}
		p, err := goja.Compile(name, s, false)
		if err != nil {
			return errors.Wrap(err, "compile error")
		}
		if _, err := rt.RunProgram(p); err != nil {
			return errors.Wrap(err, "cannot execute compiled file")
		}

	case "git+https", "git+ssh":
		name = sanitizeModuleName(name)
		dst := filepath.Join(assetDir(ctx), name)

		if err := gitClone(ctx, u.String(), dst); err != nil {
			return errors.Wrap(err, "git error")
		}

		reg := require.NewRegistry(require.WithGlobalFolders(assetDir(ctx)))
		req := reg.Enable(rt)

		m, err := req.Require(name)
		if err != nil {
			return errors.Wrap(err, "cannot require")
		}

		return rt.Set(name, m)

	default:
		return fmt.Errorf("unknown scheme %q", u.Scheme)
	}

	return nil
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

func gitClone(ctx context.Context, url, dst string) error {
	s, err := os.Stat(dst)
	if err == nil && s.IsDir() {
		return nil
	}

	url = strings.TrimPrefix(url, "git+")

	// // Prioritize using exec git.
	// cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url)
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err == nil {
	// 	return nil
	// }

	// log.Println("error executing git:", err)
	log.Println("using go-git...")

	_, err = git.PlainCloneContext(ctx, dst, false, &git.CloneOptions{
		URL:   url,
		Depth: 1,
	})
	if err != nil {
		return errors.Wrap(err, "go-git: cannot clone")
	}

	return nil
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

func trimExt(name string) string {
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

func sanitizeModuleName(name string) string {
	name = trimExt(name)

	return strings.Map(func(r rune) rune {
		if !unicode.IsLetter(r) {
			return '_'
		}
		return r
	}, name)
}
