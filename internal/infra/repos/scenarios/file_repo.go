package scenarios

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mmrzaf/sdgen/internal/domain"
	"gopkg.in/yaml.v3"
)

type Repository interface {
	List() ([]*domain.Scenario, error)
	Get(id string) (*domain.Scenario, error)
	GetByPath(path string) (*domain.Scenario, error)
}

type FileRepository struct {
	baseDir string
}

func NewFileRepository(baseDir string) *FileRepository {
	return &FileRepository{baseDir: baseDir}
}

func (r *FileRepository) List() ([]*domain.Scenario, error) {
	if _, err := os.Stat(r.baseDir); os.IsNotExist(err) {
		return []*domain.Scenario{}, nil
	}

	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, err
	}

	scenarios := make([]*domain.Scenario, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}

		path := filepath.Join(r.baseDir, entry.Name())
		scenario, err := r.loadScenario(path)
		if err != nil {
			continue
		}
		scenarios = append(scenarios, scenario)
	}

	return scenarios, nil
}

func (r *FileRepository) Get(id string) (*domain.Scenario, error) {
	scenarios, err := r.List()
	if err != nil {
		return nil, err
	}

	for _, s := range scenarios {
		if s.ID == id || s.Name == id {
			return s, nil
		}
	}

	return nil, fmt.Errorf("scenario not found: %s", id)
}

func (r *FileRepository) GetByPath(path string) (*domain.Scenario, error) {
	resolved, err := r.resolveScenarioPath(path)
	if err != nil {
		return nil, err
	}
	return r.loadScenario(resolved)
}

func (r *FileRepository) resolveScenarioPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty scenario path")
	}
	baseAbs, err := filepath.Abs(r.baseDir)
	if err != nil {
		return "", err
	}

	candidate := path
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(r.baseDir, candidate)
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("scenario path must be inside scenarios dir")
	}
	return candidateAbs, nil
}

func (r *FileRepository) loadScenario(path string) (*domain.Scenario, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var scenario domain.Scenario
	ext := filepath.Ext(path)

	if ext == ".json" {
		err = json.Unmarshal(data, &scenario)
	} else {
		err = yaml.Unmarshal(data, &scenario)
	}

	if err != nil {
		return nil, err
	}

	if scenario.ID == "" {
		scenario.ID = filepath.Base(path)
	}

	return &scenario, nil
}
