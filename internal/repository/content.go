package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/customeros/leads/internal/database"
	"github.com/customeros/leads/internal/models"
	"github.com/customeros/leads/internal/telemetry"
)

type ContentRepository interface {
	Create(ctx context.Context, content *models.Content) error
	GetByDomains(ctx context.Context, domains []string) ([]models.Content, error)
	GetByUrl(ctx context.Context, url string) (*models.Content, error)
	Update(ctx context.Context, content *models.Content) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter ContentFilter) ([]models.Content, error)
}

// ContentFilter defines filtering options for Content queries
type ContentFilter struct {
	Domain           string
	PrimaryTopic     string
	SolutionFocus    []string
	IndustryVertical string
	Limit            int
	Offset           int
}

// contentRepo implements ContentRepository
type contentRepo struct {
	writeDB *gorm.DB
	readDB  *gorm.DB
}

// NewContentRepository creates a new Content repository
func NewContentRepository(db *database.DbConnections) ContentRepository {
	return &contentRepo{
		readDB:  db.ReadDB,
		writeDB: db.WriteDB,
	}
}

// Create adds a new Content record to the database
func (r *contentRepo) Create(ctx context.Context, content *models.Content) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.Create")
	defer span.Finish()

	return r.writeDB.WithContext(ctx).Create(content).Error
}

func (r *contentRepo) GetByDomains(ctx context.Context, domains []string) ([]models.Content, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.GetByDomains")
	defer span.Finish()

	var contents []models.Content
	result := r.readDB.WithContext(ctx).Where("domain IN ?", domains).Find(&contents)
	if result.Error != nil {
		return nil, result.Error
	}

	return contents, nil
}

func (r *contentRepo) GetByUrl(ctx context.Context, url string) (*models.Content, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.GetByUrl")
	defer span.Finish()

	var content models.Content
	result := r.readDB.WithContext(ctx).Where("url = ?", url).First(&content)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &content, nil
}

func (r *contentRepo) Update(ctx context.Context, content *models.Content) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.Update")
	defer span.Finish()

	return r.writeDB.WithContext(ctx).Save(content).Error
}

// Delete removes a Content record from the database
func (r *contentRepo) Delete(ctx context.Context, id string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.Delete")
	defer span.Finish()

	return r.writeDB.WithContext(ctx).Delete(&models.Content{}, "id = ?", id).Error
}

// List retrieves Content records based on filter criteria
func (r *contentRepo) List(ctx context.Context, filter ContentFilter) ([]models.Content, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.List")
	defer span.Finish()

	var contents []models.Content
	query := r.readDB.WithContext(ctx)

	if filter.Domain != "" {
		query = query.Where("domain = ?", filter.Domain)
	}

	if filter.PrimaryTopic != "" {
		query = query.Where("primary_topic = ?", filter.PrimaryTopic)
	}

	if len(filter.SolutionFocus) > 0 {
		query = query.Where("solution_focus && ?", filter.SolutionFocus)
	}

	if filter.IndustryVertical != "" {
		query = query.Where("industry_vertical = ?", filter.IndustryVertical)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	err := query.Find(&contents).Error
	return contents, err
}
