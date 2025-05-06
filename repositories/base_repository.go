package repositories

import (
	"context"
	"errors"
	"strings"

	"davet.link/pkg/queryparams"
	"davet.link/pkg/turkishsearch"

	"gorm.io/gorm"
)

type Repository[T any] interface {
	GetAll(params queryparams.ListParams) ([]T, int64, error)
	GetByID(id uint) (*T, error)
	Create(ctx context.Context, entity *T) error
	BulkCreate(ctx context.Context, entities []T) error
	Update(ctx context.Context, id uint, data map[string]interface{}, updatedBy uint) error
	BulkUpdate(ctx context.Context, condition map[string]interface{}, data map[string]interface{}, updatedBy uint) error
	Delete(ctx context.Context, id uint) error
	BulkDelete(ctx context.Context, condition map[string]interface{}) error
	GetCount(params queryparams.ListParams) (int64, error)
}

type GenericBaseRepository[T any] struct {
	db                 *gorm.DB
	allowedSortColumns map[string]bool
}

func NewBaseRepository[T any](db *gorm.DB) *GenericBaseRepository[T] {
	return &GenericBaseRepository[T]{
		db: db,
		allowedSortColumns: map[string]bool{
			"id":         true,
			"created_at": true,
		},
	}
}

func (r *GenericBaseRepository[T]) SetAllowedSortColumns(columns []string) {
	r.allowedSortColumns = make(map[string]bool)
	for _, col := range columns {
		r.allowedSortColumns[col] = true
	}
}

func (r *GenericBaseRepository[T]) GetAll(params queryparams.ListParams) ([]T, int64, error) {
	var results []T
	var totalCount int64

	query := r.db.Model(new(T))

	if params.Name != "" {
		sqlFragment, args := turkishsearch.SQLFilter("name", params.Name)
		query = query.Where(sqlFragment, args...)
	}
	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.Type != "" {
		query = query.Where("type = ?", params.Type)
	}

	err := query.Count(&totalCount).Error
	if err != nil {
		return nil, 0, err
	}
	if totalCount == 0 {
		return results, 0, nil
	}

	sortBy := params.SortBy
	orderBy := strings.ToLower(params.OrderBy)
	if orderBy != "asc" && orderBy != "desc" {
		orderBy = queryparams.DefaultOrderBy
	}
	if _, ok := r.allowedSortColumns[sortBy]; !ok {
		sortBy = queryparams.DefaultSortBy
	}
	query = query.Order(sortBy + " " + orderBy)

	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)

	err = query.Find(&results).Error
	return results, totalCount, err
}

func (r *GenericBaseRepository[T]) GetByID(id uint) (*T, error) {
	var result T
	err := r.db.First(&result, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("kayıt bulunamadı")
	}
	return &result, err
}

func (r *GenericBaseRepository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *GenericBaseRepository[T]) BulkCreate(ctx context.Context, entities []T) error {
	return r.db.WithContext(ctx).Create(&entities).Error
}

func (r *GenericBaseRepository[T]) Update(ctx context.Context, id uint, data map[string]interface{}, updatedBy uint) error {
	if updatedBy > 0 {
		data["updated_by"] = updatedBy
	}
	result := r.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(data)
	if result.RowsAffected == 0 {
		return errors.New("kayıt bulunamadı")
	}
	return result.Error
}

func (r *GenericBaseRepository[T]) BulkUpdate(ctx context.Context, condition map[string]interface{}, data map[string]interface{}, updatedBy uint) error {
	if updatedBy > 0 {
		data["updated_by"] = updatedBy
	}
	return r.db.WithContext(ctx).Model(new(T)).Where(condition).Updates(data).Error
}

func (r *GenericBaseRepository[T]) Delete(ctx context.Context, id uint) error {
	var entity T

	userID, ok := ctx.Value("user_id").(uint)
	if !ok || userID == 0 {
		return errors.New("Delete: context içinde geçerli user_id yok")
	}

	tx := r.db.WithContext(ctx)

	if err := tx.First(&entity, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("kayıt bulunamadı")
		}
		return err
	}

	if err := tx.Model(&entity).Update("deleted_by", userID).Error; err != nil {
		return err
	}

	return tx.Delete(&entity).Error
}

func (r *GenericBaseRepository[T]) BulkDelete(ctx context.Context, condition map[string]interface{}) error {
	var entities []T

	userID, ok := ctx.Value("user_id").(uint)
	if !ok || userID == 0 {
		return errors.New("BulkDelete: context içinde geçerli user_id yok")
	}

	tx := r.db.WithContext(ctx)

	if err := tx.Where(condition).Find(&entities).Error; err != nil {
		return err
	}

	for _, entity := range entities {
		if err := tx.Model(&entity).Update("deleted_by", userID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&entity).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *GenericBaseRepository[T]) GetCount() (int64, error) {
	var totalCount int64
	err := r.db.Model(new(T)).Count(&totalCount).Error
	if err != nil {
		return 0, err
	}
	return totalCount, nil
}
