package imdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type cinemetaIMDB struct {
	httpClient *http.Client
	baseURL    string
}

type cinemetaResponse struct {
	Meta struct {
		Name string `json:"name"`
		Year string `json:"year"`
	} `json:"meta"`
}

// NewCinemetaIMDB creates a new instance of the Cinemeta implementation of the IMDB service.
func NewCinemetaIMDB() IMDB {
	return &cinemetaIMDB{
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
		baseURL: "https://v3-cinemeta.strem.io",
	}
}

// GetTitle gets a Title by its ID and type using the Stremio Cinemeta API.
func (c *cinemetaIMDB) GetTitle(ctx context.Context, titleType string, imdbID string) (*Title, error) {
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "imdb.IMDB.GetTitle")
	defer span.End()

	url := fmt.Sprintf("%s/meta/%s/%s.json", c.baseURL, titleType, imdbID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch cinemeta: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cinemeta returned status %d", resp.StatusCode)
	}

	var result cinemetaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode cinemeta response: %w", err)
	}

	if result.Meta.Name == "" {
		return nil, fmt.Errorf("cinemeta returned empty title for %s", imdbID)
	}

	// Year can be "1999" or "2011–2019" for series; extract first year.
	yearStr := strings.Split(result.Meta.Year, "–")[0]
	yearStr = strings.Split(yearStr, "-")[0]
	year, _ := strconv.Atoi(strings.TrimSpace(yearStr))

	return &Title{
		Name: result.Meta.Name,
		Year: year,
	}, nil
}
