package repo

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/saltbo/gopkg/strutil"
	"github.com/saltbo/zpan/internal/app/entity"
	"github.com/saltbo/zpan/internal/app/repo/query"
	"github.com/saltbo/zpan/internal/pkg/logger"
	"github.com/samber/lo"
	"github.com/spf13/viper"
	"gorm.io/gen"
	"gorm.io/gorm"
)

type MatterListOption struct {
	QueryPage
	Sid     int64 // storage id
	Uid     int64
	Dir     string
	Type    string
	Keyword string
	Draft   bool
}

type MatterFindWithOption struct {
	Id      int64
	Alias   string
	Deleted bool
}

type Matter interface {
	BasicOP[*entity.Matter, int64, *MatterListOption]

	FindWith(ctx context.Context, opt *MatterFindWithOption) (*entity.Matter, error)
	FindByAlias(ctx context.Context, alias string) (*entity.Matter, error)
	PathExist(ctx context.Context, path string) bool
	PathExistWithScope(ctx context.Context, path string, uid, sid, excludeID int64) bool
	Copy(ctx context.Context, id int64, to string) (*entity.Matter, error)
	Recovery(ctx context.Context, id int64) error
	GetObjects(ctx context.Context, id int64) ([]string, error)
}

var _ Matter = (*MatterDBQuery)(nil)

type MatterDBQuery struct {
	DBQuery
}

func NewMatterDBQuery(q DBQuery) *MatterDBQuery {
	return &MatterDBQuery{DBQuery: q}
}

func (db *MatterDBQuery) Find(ctx context.Context, id int64) (*entity.Matter, error) {
	return db.Q().Matter.WithContext(ctx).Where(db.Q().Matter.Id.Eq(id)).First()
}

func (db *MatterDBQuery) FindWith(ctx context.Context, opt *MatterFindWithOption) (*entity.Matter, error) {
	conds := make([]gen.Condition, 0)
	if opt.Id != 0 {
		conds = append(conds, db.Q().Matter.Id.Eq(opt.Id))
	}
	if opt.Alias != "" {
		conds = append(conds, db.Q().Matter.Alias_.Eq(opt.Alias))
	}

	q := db.Q().Matter.WithContext(ctx)
	if opt.Deleted {
		q = q.Unscoped()
	}

	return q.Where(conds...).First()
}

func (db *MatterDBQuery) FindByAlias(ctx context.Context, alias string) (*entity.Matter, error) {
	return db.Q().Matter.WithContext(ctx).Where(db.Q().Matter.Alias_.Eq(alias)).First()
}

func (db *MatterDBQuery) PathExist(ctx context.Context, filepath string) bool {
	return db.PathExistWithScope(ctx, filepath, 0, 0, 0)
}

func (db *MatterDBQuery) PathExistWithScope(ctx context.Context, filepath string, uid, sid, excludeID int64) bool {
	if filepath == "" {
		return true
	}

	var name, parent string
	if strings.HasSuffix(filepath, "/") {
		name = path.Base(filepath)
		parent = strings.TrimSuffix(filepath, name+"/")
	} else {
		parent, name = path.Split(filepath)
	}

	conds := []gen.Condition{db.Q().Matter.Name.Eq(name)}
	if parent != name {
		conds = append(conds, db.Q().Matter.Parent.Eq(strings.TrimPrefix(parent, "/")))
	}
	if uid != 0 {
		conds = append(conds, db.Q().Matter.Uid.Eq(uid))
	}
	if sid != 0 {
		conds = append(conds, db.Q().Matter.Sid.Eq(sid))
	}
	if excludeID != 0 {
		conds = append(conds, db.Q().Matter.Id.Neq(excludeID))
	}

	_, err := db.Q().Matter.WithContext(ctx).Where(conds...).First()
	return err == nil
}

func (db *MatterDBQuery) FindAll(ctx context.Context, opts *MatterListOption) ([]*entity.Matter, int64, error) {
	conds := make([]gen.Condition, 0)
	logger.Debug("FindAll with options: %+v", opts)
	if opts.Uid != 0 {
		conds = append(conds, db.Q().Matter.Uid.Eq(opts.Uid))
	}
	if opts.Sid != 0 {
		conds = append(conds, db.Q().Matter.Sid.Eq(opts.Sid))
	}

	if opts.Keyword != "" {
		conds = append(conds, db.Q().Matter.Name.Like(fmt.Sprintf("%%%s%%", opts.Keyword)))
		conds = append(conds, db.Q().Matter.Parent.Eq(opts.Dir))
	} else if !opts.Draft {
		conds = append(conds, db.Q().Matter.Parent.Eq(opts.Dir))
	}

	if opts.Type == "doc" {
		conds = append(conds, db.Q().Matter.Type.In(entity.DocTypes...))
	} else if opts.Type != "" {
		conds = append(conds, db.Q().Matter.Type.Like(fmt.Sprintf("%%%s%%", opts.Type)))
	}

	if !opts.Draft {
		conds = append(conds, db.Q().Matter.UploadedAt.IsNotNull())
	}

	q := db.Q().Matter.WithContext(ctx).Where(conds...).Order(db.Q().Matter.DirType.Desc(), db.Q().Matter.Id.Desc())

	if opts.Limit == 0 {
		matters, err := q.Find()
		return matters, int64(len(matters)), err
	}

	return q.FindByPage(opts.Offset, opts.Limit)
}

func (db *MatterDBQuery) Create(ctx context.Context, m *entity.Matter) error {
	// Auto-create missing directories if parent doesn't exist
	// This is done in a separate operation to reduce lock contention
	if m.Parent != "" {
		if err := db.createMissingDirs(ctx, m.Uid, m.Sid, m.Parent); err != nil {
			return err
		}
	}

	// Create the file/folder itself
	return db.createMatter(ctx, m)
}

// createMatter handles file/folder creation
func (db *MatterDBQuery) createMatter(ctx context.Context, m *entity.Matter) error {
	// Check if file already exists
	existingCount, err := db.Q().Matter.WithContext(ctx).
		Where(db.Q().Matter.Name.Eq(m.Name)).
		Where(db.Q().Matter.Parent.Eq(m.Parent)).
		Where(db.Q().Matter.Uid.Eq(m.Uid)).
		Where(db.Q().Matter.Sid.Eq(m.Sid)).
		Count()

	if err != nil {
		return fmt.Errorf("failed to check existing file: %v", err)
	}

	if existingCount > 0 {
		// Reject duplicate file upload with clear error message
		return fmt.Errorf("cannot upload file with the same name, please rename the file")
	}

	// Create the matter
	return db.Q().Matter.Create(m)
}

// createMissingDirs creates missing directories
func (db *MatterDBQuery) createMissingDirs(ctx context.Context, uid, sid int64, dirPath string) error {
	dirPath = strings.TrimSuffix(dirPath, "/")
	if dirPath == "" {
		return nil
	}

	sharedAllFiles := viper.GetBool("share.all_files")

	// Perform all operations within a single transaction
	return db.Q().Transaction(func(tx *query.Query) error {
		parts := strings.Split(dirPath, "/")
		currentPath := ""

		for _, part := range parts {
			if part == "" {
				continue
			}

			var parentVal string
			if currentPath == "" {
				parentVal = ""
			} else {
				parentVal = currentPath + "/"
			}

			// In shared-all-files mode, directories are global within a storage (sid).
			// They should be reused across users instead of being duplicated by uid.
			dirQuery := tx.Matter.WithContext(ctx).
				Where(tx.Matter.Name.Eq(part)).
				Where(tx.Matter.DirType.Eq(entity.DirTypeUser)).
				Where(tx.Matter.Sid.Eq(sid)).
				Where(tx.Matter.Parent.Eq(parentVal))
			if !sharedAllFiles {
				dirQuery = dirQuery.Where(tx.Matter.Uid.Eq(uid))
			}

			count, err := dirQuery.Count()

			if err != nil {
				return err
			}

			// Create directory if it doesn't exist
			if count == 0 {
				dir := &entity.Matter{
					Uid:     uid,
					Sid:     sid,
					Alias:   strutil.RandomText(16),
					Name:    part,
					DirType: entity.DirTypeUser,
					Parent:  parentVal,
				}
				if err := tx.Matter.Create(dir); err != nil {
					return err
				}
			}

			// Update path for next iteration
			if currentPath == "" {
				currentPath = part
			} else {
				currentPath = currentPath + "/" + part
			}
		}

		return nil
	})
}

func (db *MatterDBQuery) Copy(ctx context.Context, id int64, to string) (*entity.Matter, error) {
	em, err := db.Find(ctx, id)
	if err != nil {
		return nil, err
	}

	if exist := db.PathExistWithScope(ctx, path.Join(to, em.Name), em.Uid, em.Sid, em.Id); exist {
		return nil, fmt.Errorf("dir already has the same name file")
	}

	newMatter := em.Clone()
	newMatter.Parent = to
	if !em.IsDir() {
		// 如果是文件则只创建新的文件即可
		return newMatter, db.Q().Matter.Create(newMatter)
	}

	// 如果是文件夹则查找所有子文件/文件夹一起复制
	matters, err := db.findChildren(ctx, em, false)
	if err != nil {
		return nil, err
	}

	newMatters := lo.Map(matters, func(item *entity.Matter, index int) *entity.Matter {
		newMatter := em.Clone()
		newMatter.Parent = to
		return newMatter
	})

	return newMatter, db.Q().Matter.Create(newMatters...)
}

func (db *MatterDBQuery) Update(ctx context.Context, id int64, m *entity.Matter) error {
	em, err := db.Find(ctx, id)
	if err != nil {
		return err
	}

	return db.Q().Transaction(func(tx *query.Query) error {
		tq := tx.Matter.WithContext(ctx)
		if m.IsDir() {
			// 如果是目录，则需要把该目录下的子文件/目录一并修改
			emFullPath := em.FullPath()
			emPathWithoutSlash := strings.TrimLeft(emFullPath, "/")
			mFullPath := m.FullPath()

			updated := map[string]any{"parent": gorm.Expr("REPLACE(parent, ?, ?)", emFullPath, mFullPath)}
			q := tq.Where(tx.Matter.Parent.Eq(emFullPath)).Or(
				tx.Matter.Parent.Eq(emPathWithoutSlash),
			).Or(
				tx.Matter.Parent.Like(emFullPath + "%"),
			).Or(
				tx.Matter.Parent.Like(emPathWithoutSlash + "%"),
			)
			if _, err := q.Select(tx.Matter.Parent).Updates(updated); err != nil {
				return err
			}
		}

		_, err := tq.Where(tx.Matter.Id.Eq(id)).Select(tx.Matter.Name, tx.Matter.Parent, tx.Matter.Object, tx.Matter.UploadedAt).Updates(m)
		return err
	})
}

func (db *MatterDBQuery) Delete(ctx context.Context, id int64) error {
	m, err := db.Find(ctx, id)
	if err != nil {
		return err
	}

	m.TrashedBy = uuid.New().String()
	return db.Q().Transaction(func(tx *query.Query) error {
		tq := tx.Matter.WithContext(ctx)
		if m.IsDir() {
			// 如果是目录，则需要把该目录下的子文件/目录一并删除
			fullPath := m.FullPath()
			pathWithoutLeadingSlash := strings.TrimLeft(fullPath, "/")

			// Build query conditions that handle different parent path formats
			q := tq.Where(tx.Matter.Parent.Eq(fullPath)).Or(
				tx.Matter.Parent.Eq(pathWithoutLeadingSlash),
			).Or(
				tx.Matter.Parent.Like(fullPath + "%"),
			).Or(
				tx.Matter.Parent.Like(pathWithoutLeadingSlash + "%"),
			)

			if _, err := q.Update(tx.Matter.TrashedBy, m.TrashedBy); err != nil {
				return err
			}

			q = tq.Where(tx.Matter.Parent.Eq(fullPath)).Or(
				tx.Matter.Parent.Eq(pathWithoutLeadingSlash),
			).Or(
				tx.Matter.Parent.Like(fullPath + "%"),
			).Or(
				tx.Matter.Parent.Like(pathWithoutLeadingSlash + "%"),
			)

			if _, err := q.Delete(); err != nil {
				return err
			}
		}

		if _, err := tq.Where(tx.Matter.Id.Eq(m.Id)).Select(tx.Matter.TrashedBy).Updates(m); err != nil {
			return err
		}
		_, err := tq.Delete(m)
		return err
	})
}

func (db *MatterDBQuery) Recovery(ctx context.Context, id int64) error {
	m, err := db.Q().Matter.WithContext(ctx).Unscoped().Where(db.Q().Matter.Id.Eq(id)).First()
	if err != nil {
		return err
	}

	if !db.PathExistWithScope(ctx, m.Parent, m.Uid, m.Sid, m.Id) {
		return fmt.Errorf("recovery: file parent[%s] not found", m.Parent)
	}

	_, err = db.Q().Matter.WithContext(ctx).Unscoped().Where(db.Q().Matter.TrashedBy.Eq(m.TrashedBy)).
		UpdateSimple(db.Q().Matter.TrashedBy.Value(""), db.Q().Matter.DeletedAt.Null())
	return err
}

func (db *MatterDBQuery) GetObjects(ctx context.Context, id int64) ([]string, error) {
	m, err := db.FindWith(ctx, &MatterFindWithOption{Id: id, Deleted: true})
	if err != nil {
		return nil, err
	}

	if !m.IsDir() {
		return []string{m.Object}, nil
	}

	matters, err := db.findChildren(ctx, m, true)
	if err != nil {
		return nil, err
	}

	return lo.Map(lo.Filter(append(matters, m), func(item *entity.Matter, index int) bool {
		return item.Object != ""
	}), func(item *entity.Matter, index int) string {
		return item.Object
	}), nil
}

func (db *MatterDBQuery) findChildren(ctx context.Context, m *entity.Matter, withDeleted bool) ([]*entity.Matter, error) {
	q := db.Q().Matter.WithContext(ctx)
	if withDeleted {
		q = q.Unscoped()
	}

	// Build query conditions that handle different parent path formats
	fullPath := m.FullPath()
	pathWithoutLeadingSlash := strings.TrimLeft(fullPath, "/")

	// Find both direct children and nested items
	// Handle cases where parent might be stored in different formats
	return q.Where(
		db.Q().Matter.Parent.Eq(fullPath),
	).Or(
		db.Q().Matter.Parent.Eq(pathWithoutLeadingSlash),
	).Or(
		db.Q().Matter.Parent.Like(fullPath + "%"),
	).Or(
		db.Q().Matter.Parent.Like(pathWithoutLeadingSlash + "%"),
	).Find()
}
