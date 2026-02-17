package targets

import "github.com/mmrzaf/sdgen/internal/domain"

type Repository interface {
	List() ([]*domain.TargetConfig, error)
	Get(id string) (*domain.TargetConfig, error)

	Create(t *domain.TargetConfig) error
	Update(t *domain.TargetConfig) error
	Delete(id string) error

	RecordCheck(c *domain.TargetCheck) error
	ListChecks(targetID string, limit int) ([]*domain.TargetCheck, error)
}
