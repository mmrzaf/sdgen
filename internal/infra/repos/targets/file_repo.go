package targets

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mmrzaf/sdgen/internal/domain"
	"gopkg.in/yaml.v3"
)

type Repository interface {
	List() ([]*domain.TargetConfig, error)
	Get(id string) (*domain.TargetConfig, error)
	GetByPath(path string) (*domain.TargetConfig, error)
}

type FileRepository struct {
	baseDir string
}

func NewFileRepository(baseDir string) *FileRepository {
	return &FileRepository{baseDir: baseDir}
}

func (r *FileRepository) List() ([]*domain.TargetConfig, error) {
	if _, err := os.Stat(r.baseDir); os.IsNotExist(err) {
		return []*domain.TargetConfig{}, nil
	}

	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, err
	}

	targets := make([]*domain.TargetConfig, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" && ext != ".json" {
			continue
		}

		path := filepath.Join(r.baseDir, entry.Name())
		target, err := r.loadTarget(path)
		if err != nil {
			continue
		}
		targets = append(targets, target)
	}

	return targets, nil
}

func (r *FileRepository) Get(id string) (*domain.TargetConfig, error) {
	targets, err := r.List()
	if err != nil {
		return nil, err
	}

	for _, t := range targets {
		if t.ID == id || t.Name == id {
			return t, nil
		}
	}

	return nil, fmt.Errorf("target not found: %s", id)
}

func (r *FileRepository) GetByPath(path string) (*domain.TargetConfig, error) {
	return r.loadTarget(path)
}

func (r *FileRepository) loadTarget(path string) (*domain.TargetConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var target domain.TargetConfig
	ext := filepath.Ext(path)

	if ext == ".json" {
		err = json.Unmarshal(data, &target)
	} else {
		err = yaml.Unmarshal(data, &target)
	}

	if err != nil {
		return nil, err
	}

	if target.ID == "" {
		target.ID = filepath.Base(path)
	}

	return &target, nil
}
