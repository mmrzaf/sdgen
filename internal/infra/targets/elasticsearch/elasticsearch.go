package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mmrzaf/sdgen/internal/domain"
)

type ElasticsearchTarget struct {
	baseURL string
	client  *http.Client
}

func NewElasticsearchTarget(dsn string) *ElasticsearchTarget {
	return &ElasticsearchTarget{baseURL: normalizeURL(dsn)}
}

func (t *ElasticsearchTarget) Connect() error {
	t.client = &http.Client{Timeout: 15 * time.Second}
	resp, err := t.client.Get(t.baseURL + "/")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch ping failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (t *ElasticsearchTarget) Close() error { return nil }

func (t *ElasticsearchTarget) CreateTableIfNotExists(entity *domain.Entity) error {
	indexName := toIndexName(entity.TargetTable)
	req, err := http.NewRequest(http.MethodPut, t.baseURL+"/"+indexName, nil)
	if err != nil {
		return err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "resource_already_exists_exception") {
		return nil
	}
	return fmt.Errorf("elasticsearch create index failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (t *ElasticsearchTarget) TruncateTable(tableName string) error {
	indexName := toIndexName(tableName)
	payload := []byte(`{"query":{"match_all":{}}}`)
	req, err := http.NewRequest(http.MethodPost, t.baseURL+"/"+indexName+"/_delete_by_query", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("elasticsearch truncate failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (t *ElasticsearchTarget) InsertBatch(tableName string, columns []string, rows [][]interface{}) error {
	if len(rows) == 0 {
		return nil
	}
	indexName := toIndexName(tableName)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, row := range rows {
		if err := enc.Encode(map[string]any{"index": map[string]string{"_index": indexName}}); err != nil {
			return err
		}
		doc := map[string]any{}
		for i, col := range columns {
			doc[col] = row[i]
		}
		if err := enc.Encode(doc); err != nil {
			return err
		}
	}
	req, err := http.NewRequest(http.MethodPost, t.baseURL+"/_bulk", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("elasticsearch bulk insert failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var bulkResp struct {
		Errors bool `json:"errors"`
	}
	_ = json.Unmarshal(body, &bulkResp)
	if bulkResp.Errors {
		return fmt.Errorf("elasticsearch bulk insert returned errors")
	}
	return nil
}

func normalizeURL(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "http://localhost:9200"
	}
	if strings.HasPrefix(dsn, "http://") || strings.HasPrefix(dsn, "https://") {
		return strings.TrimRight(dsn, "/")
	}
	return "http://" + strings.TrimRight(dsn, "/")
}

func toIndexName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	return url.PathEscape(name)
}

func GetServerVersion(dsn string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	base := normalizeURL(dsn)
	resp, err := client.Get(base + "/")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var root struct {
		Version struct {
			Number string `json:"number"`
		} `json:"version"`
	}
	if err := json.Unmarshal(body, &root); err != nil {
		return "", err
	}
	return root.Version.Number, nil
}
