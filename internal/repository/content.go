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
	UpdateClassificationFields(ctx context.Context, content *models.Content) error
	UpdateIntentScores(ctx context.Context, content *models.Content) error
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

	err := r.writeDB.WithContext(ctx).Create(content).Error
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (r *contentRepo) GetByDomains(ctx context.Context, domains []string) ([]models.Content, error) {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.GetByDomains")
	defer span.Finish()

	var contents []models.Content
	result := r.readDB.WithContext(ctx).Where("domain IN ?", domains).Find(&contents)
	if result.Error != nil {
		span.TraceError(result.Error)
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
		span.TraceError(result.Error)
		return nil, result.Error
	}
	return &content, nil
}

// UpdateClassificationFields updates only the specified classification fields
func (r *contentRepo) UpdateClassificationFields(ctx context.Context, content *models.Content) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.UpdateClassificationFields")
	defer span.Finish()

	if content.ID == "" {
		err := errors.New("ContentID cannot be empty")
		span.TraceError(err)
		return err
	}

	// Build updates map with only non-empty fields
	updates := make(map[string]any)

	if content.PrimaryTopic != "" {
		updates["primary_topic"] = content.PrimaryTopic
	}
	if len(content.SecondaryTopics) > 0 {
		updates["secondary_topics"] = content.SecondaryTopics
	}
	if len(content.SolutionFocus) > 0 {
		updates["solution_focus"] = content.SolutionFocus
	}
	if content.ContentType != "" {
		updates["content_type"] = content.ContentType
	}
	if content.IndustryVertical != "" {
		updates["industry_vertical"] = content.IndustryVertical
	}
	if len(content.KeyPainPoints) > 0 {
		updates["key_pain_points"] = content.KeyPainPoints
	}
	if content.ValueProposition != "" {
		updates["value_proposition"] = content.ValueProposition
	}
	if len(content.ReferencedCustomers) > 0 {
		updates["referenced_customers"] = content.ReferencedCustomers
	}

	if len(updates) == 0 {
		return nil // nothing to update
	}

	err := r.writeDB.WithContext(ctx).
		Model(&models.Content{}).
		Where("id = ?", content.ID).
		Updates(updates).
		Error
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

func (r *contentRepo) UpdateIntentScores(ctx context.Context, content *models.Content) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.UpdateIntentScores")
	defer span.Finish()

	if content.ID == "" {
		err := errors.New("ContentID cannot be empty")
		span.TraceError(err)
		return err
	}

	// Build updates map with only non-empty fields
	updates := make(map[string]any)

	if content.ProblemRecognitionScore > 0 && content.ProblemRecognitionScore < 6 {
		updates["problem_recognition_score"] = content.ProblemRecognitionScore
	}

	if content.SolutionResearchScore > 0 && content.SolutionResearchScore < 6 {
		updates["solution_research_score"] = content.SolutionResearchScore
	}

	if content.EvaluationScore > 0 && content.EvaluationScore < 6 {
		updates["evaluation_score"] = content.EvaluationScore
	}

	if content.PurchaseReadinessScore > 0 && content.PurchaseReadinessScore < 6 {
		updates["purchase_readiness_score"] = content.PurchaseReadinessScore
	}

	if len(updates) == 0 {
		return nil // nothing to update
	}

	return r.writeDB.WithContext(ctx).
		Model(&models.Content{}).
		Where("id = ?", content.ID).
		Updates(updates).
		Error
}

func (r *contentRepo) Update(ctx context.Context, content *models.Content) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.Update")
	defer span.Finish()

	err := r.writeDB.WithContext(ctx).Save(content).Error
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
}

// Delete removes a Content record from the database
func (r *contentRepo) Delete(ctx context.Context, id string) error {
	span, ctx := telemetry.StartPostgresSpan(ctx, "contentRepo.Delete")
	defer span.Finish()

	err := r.writeDB.WithContext(ctx).Delete(&models.Content{}, "id = ?", id).Error
	if err != nil {
		span.TraceError(err)
		return err
	}

	return nil
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
	if err != nil {
		span.TraceError(err)
		return nil, err
	}

	return contents, nil
}
