package storage

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/app/repo"
	"github.com/saltbo/zpan/internal/pkg/logger"
	"github.com/saltbo/zpan/internal/pkg/provider"
)

// StorageScanner handles scanning and importing existing files from cloud storage
type CloudStorageScanner struct {
	storageRepo  repo.Storage
	matterRepo   repo.Matter
	cloudStorage *CloudStorage
}

// CloudStorageScanResult represents the result of a storage scan operation
type CloudStorageScanResult struct {
	Total   int64
	Created int64
	Skipped int64
	Failed  int64
	Errors  []string
}

func NewCloudStorageScanner(storageRepo repo.Storage, matterRepo repo.Matter, cloudStorage *CloudStorage) *CloudStorageScanner {
	return &CloudStorageScanner{
		storageRepo:  storageRepo,
		matterRepo:   matterRepo,
		cloudStorage: cloudStorage,
	}
}

// ScanStorageObjects scans and imports existing files from cloud storage
// uid: user who owns these files
// sid: storage ID
// prefix: optional prefix to scan (e.g., specific directory in storage)
func (s *CloudStorageScanner) ScanObjects(ctx context.Context, uid int64, sid int64, prefix string) (*CloudStorageScanResult, error) {
	result := &CloudStorageScanResult{
		Errors: make([]string, 0),
	}

	// Get storage configuration
	storage, err := s.storageRepo.Find(ctx, sid)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage: %w", err)
	}

	// Get provider
	p, err := s.cloudStorage.GetProviderByStorage(storage)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider: %w", err)
	}

	// Build the scan prefix from rootPath and user prefix
	scanPrefix := storage.RootPath
	if prefix != "" {
		scanPrefix = filepath.Join(scanPrefix, strings.TrimPrefix(prefix, "/"))
	}

	logger.Info("Starting storage scan", "storage_id", sid, "user_id", uid, "prefix", scanPrefix)

	// List objects from storage
	objects, err := p.List(scanPrefix)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	result.Total = int64(len(objects))

	// Process each object
	for _, obj := range objects {
		// Skip empty keys
		if obj.Key == "" {
			result.Skipped++
			continue
		}

		// Parse the object path to extract directory and filename
		matter := s.parseObjectToMatter(uid, sid, obj, storage)
		if matter == nil {
			result.Skipped++
			continue
		}

		// Check if matter already exists
		if s.matterRepo.PathExist(ctx, matter.FullPath()) {
			result.Skipped++
			logger.Debug("Matter already exists, skipping", "path", matter.FullPath())
			continue
		}

		// Ensure parent directory exists
		if err := s.ensureParentDir(ctx, uid, sid, matter.Parent); err != nil {
			result.Failed++
			errMsg := fmt.Sprintf("failed to create parent dir %s: %v", matter.Parent, err)
			result.Errors = append(result.Errors, errMsg)
			logger.Error(errMsg)
			continue
		}

		// Create the matter record
		if err := s.matterRepo.Create(ctx, matter); err != nil {
			result.Failed++
			errMsg := fmt.Sprintf("failed to create matter %s: %v", matter.FullPath(), err)
			result.Errors = append(result.Errors, errMsg)
			logger.Error(errMsg)
			continue
		}

		result.Created++
		logger.Debug("Created matter from object", "path", matter.FullPath(), "object", obj.Key)
	}

	logger.Info("Storage scan completed", "total", result.Total, "created", result.Created, "skipped", result.Skipped, "failed", result.Failed)

	return result, nil
}

// parseObjectToMatter converts a storage object to a Matter entity
// Returns nil if the object should be skipped
func (s *CloudStorageScanner) parseObjectToMatter(uid, sid int64, obj provider.Object, storage *entity.Storage) *entity.Matter {
	// Remove root path prefix from object key
	objectPath := obj.Key
	if storage.RootPath != "" && strings.HasPrefix(objectPath, storage.RootPath) {
		objectPath = strings.TrimPrefix(objectPath, storage.RootPath)
		objectPath = strings.TrimLeft(objectPath, "/")
	}

	// Skip empty paths after prefix removal
	if objectPath == "" {
		return nil
	}

	// Extract directory and filename, normalize path separators
	objectPath = strings.ReplaceAll(objectPath, "\\", "/")
	dir := filepath.Dir(objectPath)
	filename := filepath.Base(objectPath)

	// Format directory path: normalize to "dir/" format or empty string for root
	if dir != "." && dir != "" {
		dir = strings.Trim(dir, "/")
		dir += "/"
	} else {
		dir = ""
	}

	// Create matter entity
	matter := entity.NewMatter(uid, sid, filename)
	matter.Parent = dir
	matter.Object = obj.Key
	matter.Type = s.detectContentType(strings.ToLower(filepath.Ext(filename)))
	matter.DirType = 0 // Treat as file (directories detected by keys ending with "/" in storage)
	matter.SetUploadedAt()

	return matter
}

// ensureParentDir creates parent directories if they don't exist
func (s *CloudStorageScanner) ensureParentDir(ctx context.Context, uid, sid int64, parent string) error {
	if parent == "" {
		return nil
	}

	// Check if parent directory exists
	if s.matterRepo.PathExist(ctx, parent) {
		return nil
	}

	// Extract path components and create directories recursively
	parts := strings.Split(strings.Trim(parent, "/"), "/")
	currentPath := "/"

	for _, part := range parts {
		if part == "" {
			continue
		}

		currentPath = filepath.Join(currentPath, part) + "/"

		if s.matterRepo.PathExist(ctx, currentPath) {
			continue
		}

		// Create directory
		dirMatter := entity.NewMatter(uid, sid, part)
		parentPath := strings.TrimSuffix(currentPath, part+"/")
		
		// Convert parentPath format: remove leading "/" and keep trailing "/"
		if parentPath == "/" {
			dirMatter.Parent = ""
		} else {
			// Remove leading "/" but keep trailing "/"
			dirMatter.Parent = strings.TrimPrefix(parentPath, "/")
			if !strings.HasSuffix(dirMatter.Parent, "/") {
				dirMatter.Parent += "/"
			}
		}
		
		dirMatter.Type = "application/x-directory"
		dirMatter.DirType = entity.DirTypeUser
		dirMatter.Object = "" // directories don't have objects
		dirMatter.SetUploadedAt()

		if err := s.matterRepo.Create(ctx, dirMatter); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", currentPath, err)
		}

		logger.Debug("Created parent directory", "path", currentPath)
	}

	return nil
}

// matterExists checks if a matter with the given path already exists (deprecated, use PathExist)
func (s *CloudStorageScanner) matterExists(ctx context.Context, uid, sid int64, parent, name string) bool {
	matters, _, err := s.matterRepo.FindAll(ctx, &repo.MatterListOption{
		Uid: uid,
		Sid: sid,
	})
	if err != nil {
		return false
	}

	fullPath := filepath.Join(parent, name)
	for _, m := range matters {
		if m.FullPath() == fullPath {
			return true
		}
	}

	return false
}

// detectContentType detects content type based on file extension
func (s *CloudStorageScanner) detectContentType(ext string) string {
	contentTypes := map[string]string{
		".pdf":  "application/pdf",
		".txt":  "text/plain",
		".json": "application/json",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".zip":  "application/zip",
		".rar":  "application/x-rar-compressed",
		".gz":   "application/gzip",
		".tar":  "application/x-tar",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}

	if ct, ok := contentTypes[ext]; ok {
		return ct
	}

	return "application/octet-stream"
}
