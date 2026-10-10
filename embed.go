// Package web embeds the survey page (the web directory) so the server ships
// as a single binary. It lives at the module root because go:embed cannot
// reach parent directories.
package web

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed web
var files embed.FS

// Site returns the survey page files, rooted at the web directory.
func Site() (fs.FS, error) {
	site, err := fs.Sub(files, "web")
	if err != nil {
		return nil, fmt.Errorf("web: %w", err)
	}
	return site, nil
}
