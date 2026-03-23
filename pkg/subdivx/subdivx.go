package subdivx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/go-unarr"
	"go.opentelemetry.io/otel/trace"
)

// Token represents the validation token and associated cookie for Subdivx API requests.
type Token struct {
	Token  string
	Cookie *http.Cookie
}

// Subtitles holds the total number of records and the corresponding IDs of a subset of them.
type Subtitles struct {
	TotalRecords int
	Subtitles    []*Subtitle
}
type Subtitle struct {
	ID               int
	Title            string
	Description      string
	DescriptionWords []string
}

// SubtitleContents holds content of a subtitle.
type SubtitleContents struct {
	Name string
	Data []byte
}

// Subdivx defines the methods to interact with the Subdivx service.
type Subdivx interface {
	// GetToken retrieves the validation token and associated cookie for Subdivx API requests.
	GetToken(ctx context.Context) (*Token, error)
	// GetSubtitles fetches subtitles for a given title.
	GetSubtitles(ctx context.Context, token *Token, title string) (*Subtitles, error)
	// GetSubtitle retrieves a specific subtitle file contents by its ID.
	GetSubtitle(ctx context.Context, ID string) (*SubtitleContents, error)
}

// NewSubdivx creates a new instance of the Subdivx service.
// flareSolverrURL is the URL of a FlareSolverr instance (e.g., "http://flaresolverr:8191").
// If empty, requests are made directly without Cloudflare bypass.
func NewSubdivx(flareSolverrURL string) Subdivx {
	jar, _ := cookiejar.New(nil)

	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConns = 100
	t.MaxConnsPerHost = 100
	t.MaxIdleConnsPerHost = 100

	return &subdivx{
		httpClient: &http.Client{
			Timeout:   time.Second * 30,
			Transport: t,
			Jar:       jar,
		},
		flareSolverrClient: &http.Client{
			Timeout: 150 * time.Second,
		},
		flareSolverrURL:  flareSolverrURL,
		versionREMatcher: regexp.MustCompile(`>v([0-9.a-z]+)<`),
		baseURL:          "https://www.subdivx.com",
		userAgent:        "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36",
	}
}

type subdivx struct {
	httpClient         *http.Client
	flareSolverrClient *http.Client
	flareSolverrURL    string
	versionREMatcher   *regexp.Regexp
	baseURL            string

	cfMutex   sync.Mutex
	userAgent string
	cfExpiry  time.Time
}

// ensureCFClearance obtains cf_clearance cookies via FlareSolverr if needed.
func (s *subdivx) ensureCFClearance(ctx context.Context) error {
	if s.flareSolverrURL == "" {
		return nil
	}

	s.cfMutex.Lock()
	defer s.cfMutex.Unlock()

	if time.Now().Before(s.cfExpiry) {
		return nil // cookies still valid
	}

	slog.InfoContext(ctx, "Refreshing Cloudflare cookies via FlareSolverr")

	reqBody, _ := json.Marshal(map[string]interface{}{
		"cmd":        "request.get",
		"url":        s.baseURL,
		"maxTimeout": 120000,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.flareSolverrURL+"/v1", bytes.NewReader(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create FlareSolverr request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := s.flareSolverrClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call FlareSolverr: %w", err)
	}
	defer res.Body.Close()

	var fsResp struct {
		Status   string `json:"status"`
		Message  string `json:"message"`
		Solution struct {
			UserAgent string `json:"userAgent"`
			Cookies   []struct {
				Name   string  `json:"name"`
				Value  string  `json:"value"`
				Domain string  `json:"domain"`
				Path   string  `json:"path"`
				Expiry float64 `json:"expiry"`
				Secure bool    `json:"secure"`
			} `json:"cookies"`
		} `json:"solution"`
	}

	if err := json.NewDecoder(res.Body).Decode(&fsResp); err != nil {
		return fmt.Errorf("failed to decode FlareSolverr response: %w", err)
	}

	if fsResp.Status != "ok" {
		return fmt.Errorf("FlareSolverr failed: %s", fsResp.Message)
	}

	// Apply cookies to the jar
	baseURL, _ := url.Parse(s.baseURL)
	var cookies []*http.Cookie
	for _, c := range fsResp.Solution.Cookies {
		cookies = append(cookies, &http.Cookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
			Path:   c.Path,
			Secure: c.Secure,
		})
	}
	s.httpClient.Jar.SetCookies(baseURL, cookies)

	if fsResp.Solution.UserAgent != "" {
		s.userAgent = fsResp.Solution.UserAgent
	}

	// Refresh cookies 5 minutes before they would expire (typically ~30 min)
	s.cfExpiry = time.Now().Add(25 * time.Minute)

	slog.InfoContext(ctx, "Cloudflare cookies refreshed successfully")
	return nil
}

// setHeaders sets browser-like headers on the request.
func (s *subdivx) setHeaders(req *http.Request) {
	req.Header.Set("User-Agent", s.userAgent)
	req.Header.Set("Accept-Language", "es-AR,es;q=0.9,en;q=0.8")
}

// GetToken retrieves the validation token and associated cookie for Subdivx API requests.
func (s *subdivx) GetToken(ctx context.Context) (*Token, error) {

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "subdivx.Subdivx.GetToken")
	defer span.End()

	if err := s.ensureCFClearance(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure CF clearance: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/inc/gt.php?gt=1", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to http.NewRequestWithContext: %w", err)
	}
	s.setHeaders(req)

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to http.Client.Get: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusForbidden {
		// Cloudflare cookies expired, force refresh on next call
		s.cfMutex.Lock()
		s.cfExpiry = time.Time{}
		s.cfMutex.Unlock()
		return nil, fmt.Errorf("subdivx returned 403, Cloudflare cookies may have expired")
	}

	tokenResponse := struct {
		Token string `json:"token"`
	}{}

	err = json.NewDecoder(res.Body).Decode(&tokenResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to json.NewDecoder.Decode: %w", err)
	}

	// Try to parse the sdx cookie from Set-Cookie header.
	// With a cookie jar, the cookie is managed automatically, so this may be empty.
	var cookie *http.Cookie
	if setCookie := res.Header.Get("Set-Cookie"); setCookie != "" {
		cookie, _ = http.ParseSetCookie(setCookie)
	}

	return &Token{
		Token:  tokenResponse.Token,
		Cookie: cookie,
	}, nil
}

// GetSubtitles fetches subtitles for a given title.
func (s *subdivx) GetSubtitles(ctx context.Context, token *Token, title string) (*Subtitles, error) {

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "subdivx.Subdivx.GetSubtitles")
	defer span.End()

	if err := s.ensureCFClearance(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure CF clearance: %w", err)
	}

	webVersion, err := func() (string, error) {

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL, nil)
		if err != nil {
			return "", fmt.Errorf("failed to http.NewRequestWithContext: %w", err)
		}
		s.setHeaders(req)

		res, err := s.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to http.Client.Get: %w", err)
		}
		defer res.Body.Close()

		html, err := io.ReadAll(res.Body)
		if err != nil {
			return "", fmt.Errorf("failed to io.ReadAll: %w", err)
		}

		matches := s.versionREMatcher.FindSubmatch(html)
		if matches == nil || len(matches) != 2 {
			return "", fmt.Errorf("failed to regexp.Regexp.FindSubmatch")
		}

		return string(bytes.ReplaceAll(matches[1], []byte("."), []byte(""))), nil
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch web version: %w", err)
	}

	formData := url.Values{}
	formData.Set("tabla", "resultados")
	formData.Set("filtros", "")
	formData.Set("buscar"+webVersion, title)
	formData.Set("token", token.Token)

	encodedForm := formData.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+`/inc/ajax.php`, strings.NewReader(encodedForm))
	if err != nil {
		return nil, fmt.Errorf("failed to http.NewRequestWithContext: %w", err)
	}
	s.setHeaders(req)

	if token.Cookie != nil {
		req.Header.Set("Cookie", token.Cookie.String())
	}
	req.Header.Set("Content-Type", `application/x-www-form-urlencoded; charset=UTF-8`)

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to http.Client.Do: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		return nil, fmt.Errorf("invalid status code: %d", res.StatusCode)
	}

	subdivxResponse := struct {
		ITotalRecords int `json:"iTotalRecords"`
		AaData        []struct {
			ID          int    `json:"id"`
			Title       string `json:"titulo"`
			Description string `json:"descripcion"`
		} `json:"aaData"`
	}{}

	err = json.NewDecoder(res.Body).Decode(&subdivxResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to json.NewDecoder.Decode: %w", err)
	}

	subtitles := &Subtitles{
		TotalRecords: subdivxResponse.ITotalRecords,
		Subtitles:    make([]*Subtitle, 0, subdivxResponse.ITotalRecords),
	}
	for _, aaData := range subdivxResponse.AaData {
		subtitles.Subtitles = append(subtitles.Subtitles, &Subtitle{
			ID:               aaData.ID,
			Title:            aaData.Title,
			Description:      aaData.Description,
			DescriptionWords: alphaNumericDistinctLowercaseWords(aaData.Title + " " + aaData.Description),
		})
	}

	return subtitles, nil
}

// GetSubtitle retrieves the subtitles archive for the specified ID, extracts it and returns the content of the first SRT file on it
func (s *subdivx) GetSubtitle(ctx context.Context, ID string) (*SubtitleContents, error) {

	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "subdivx.Subdivx.GetSubtitle")
	defer span.End()

	if err := s.ensureCFClearance(ctx); err != nil {
		return nil, fmt.Errorf("failed to ensure CF clearance: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/descargar.php?id="+ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to http.NewRequestWithContext: %w", err)
	}
	s.setHeaders(req)

	res, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to http.Client.Do: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid status code: %d", res.StatusCode)
	}

	lr := LimitReader(res.Body, 500*1024, ErrReadBeyondLimit)

	archiveBytes, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("failed to io.ReadAll: %w", err)
	}

	if len(archiveBytes) == 0 {
		return nil, errors.New("archive is empty")
	}

	ar, err := unarr.NewArchiveFromMemory(archiveBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to unarr.NewArchiveFromReader: %w", err)
	}
	defer ar.Close()

	var sub *SubtitleContents
	for {
		err = ar.Entry()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, fmt.Errorf("failed to unarr.Archive.Entry: %w", err)
			}
		}
		fName := ar.Name()
		if isSubtitle(ar.Name()) {
			fData, err := ar.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("failed to unarr.Archive.ReadAll: %w", err)
			}
			sub = &SubtitleContents{
				Name: fName,
				Data: fData,
			}
		}
	}

	if sub == nil {
		return nil, fmt.Errorf("no subtitle file found in archive")
	}

	return sub, nil
}

func isSubtitle(filename string) bool {
	lcFilename := strings.ToLower(filename)
	if len(lcFilename) > 4 {
		ext := lcFilename[len(lcFilename)-4:]
		switch ext {
		case ".srt", ".sub", ".ssa":
			return true
		}
	}
	return false
}
