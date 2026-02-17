package runs

import "github.com/mmrzaf/sdgen/internal/domain"

// Repository stores run metadata and progress for the sdgen control plane DB.
type Repository interface {
	Create(run *domain.Run) error
	Update(run *domain.Run) error
	Get(id string) (*domain.Run, error)
	List(limit int, status string) ([]*domain.Run, error)
	UpdateStatus(id string, status domain.RunStatus, errMsg string, stats *domain.RunStats) error
	UpdateProgress(id string, rowsGenerated, rowsTotal int64, entitiesDone, entitiesTotal int, currentEntity string) error
	AppendRunLog(runID, level, message string) error
	ListRunLogs(runID string, limit int) ([]*domain.RunLog, error)
}
