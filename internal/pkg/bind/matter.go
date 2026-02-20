package bind

import (
	"mime"
	"path/filepath"
	"strings"

	"github.com/saltbo/zpan/internal/app/entity"
)

type QueryFiles struct {
	QueryPage
	Sid     int64  `form:"sid" binding:"required"`
	Dir     string `form:"dir"`
	Type    string `form:"type"`
	Keyword string `form:"kw"`
}

type BodyMatter struct {
	Sid        int64  `json:"sid" binding:"required"`
	Name       string `json:"name" binding:"required"`         // File/folder name only
	RelPath    string `json:"rel_path"`                        // Relative path for nested uploads (e.g. "folder/subfolder")
	IsDir      bool   `json:"is_dir"`
	Dir        string `json:"dir"`                             // Base directory path
	Type       string `json:"type"`
	Size       int64  `json:"size"`
}

func (p *BodyMatter) ToMatter(uid int64) *entity.Matter {
	detectType := func(name string) string {
		cType := mime.TypeByExtension(filepath.Ext(name))
		if cType != "" {
			return cType
		}

		return "application/octet-stream"
	}

	// Handle case where RelPath is provided for nested directory uploads
	// RelPath takes precedence over any path separators in Name
	name := p.Name
	dir := p.Dir

	if p.RelPath != "" {
		// RelPath is explicitly provided for nested uploads
		// e.g., RelPath="folder/subfolder" with Name="file.txt"
		relPath := strings.TrimSuffix(p.RelPath, "/")
		if relPath != "" {
			dir = dir + relPath + "/"
		}
	} else {
		// Fallback: handle case where Name contains path separators (legacy support)
		// e.g., Name="script/domain_ping_test.sh" (from web folder uploads)
		parts := strings.Split(filepath.Clean(name), "/")
		if len(parts) > 1 {
			name = parts[len(parts)-1]
			subDir := strings.Join(parts[:len(parts)-1], "/") + "/"
			dir = dir + subDir
		}
	}

	m := entity.NewMatter(uid, p.Sid, name)
	m.Type = p.Type
	m.Size = p.Size
	m.Parent = dir
	if p.IsDir {
		m.DirType = entity.DirTypeUser
	} else if p.Type == "" {
		m.Type = detectType(p.Name)
	}
	return m
}

type BodyFileRename struct {
	NewName string `json:"name" binding:"required"`
}

type BodyFileMove struct {
	NewDir string `json:"dir"`
}

type BodyFileCopy struct {
	NewPath string `json:"path" binding:"required"`
}
