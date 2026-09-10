package browserd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultClientTimeout = 5 * time.Minute

type Config struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type responseEnvelope[T any] struct {
	Data  *T             `json:"data"`
	Error *responseError `json:"error"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewClient(cfg Config) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, Error{Code: "browserd_base_url_required", Message: "browserd base URL is required"}
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, Error{Code: "browserd_base_url_invalid", Message: "browserd base URL is invalid"}
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultClientTimeout}
	}
	return &Client{baseURL: baseURL, apiKey: strings.TrimSpace(cfg.APIKey), httpClient: httpClient}, nil
}

func (c *Client) CreateSession(ctx context.Context, input CreateSessionInput) (Session, error) {
	var out Session
	err := c.doJSON(ctx, http.MethodPost, "/v1/sessions", input, &out)
	return out, err
}

func (c *Client) Navigate(ctx context.Context, runtimeSessionID string, input NavigateInput) (NavigateResult, error) {
	var out NavigateResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "navigate"), input, &out)
	if err != nil {
		var response Error
		if input.IncludeSnapshot && AsError(err, &response) && response.Code == "browserd_response_invalid" && response.StatusCode >= 200 && response.StatusCode < 300 {
			return NavigateResult{}, Error{Code: "BROWSERD_SNAPSHOT_REQUIRED", Message: "navigation did not return the requested valid snapshot", Operation: http.MethodPost, Path: sessionPath(runtimeSessionID, "navigate"), StatusCode: response.StatusCode, Cause: err}
		}
		return NavigateResult{}, err
	}
	if input.IncludeSnapshot {
		err = ValidateSnapshotResult(out.Snapshot)
		if err == nil && out.URL != out.Snapshot.Page["url"] {
			err = fmt.Errorf("navigation URL does not match snapshot URL")
		}
		if err != nil {
			return NavigateResult{}, Error{Code: "BROWSERD_SNAPSHOT_REQUIRED", Message: "navigation did not return the requested valid snapshot", Operation: http.MethodPost, Path: sessionPath(runtimeSessionID, "navigate"), Cause: err}
		}
	}
	return out, nil
}

func (c *Client) Snapshot(ctx context.Context, runtimeSessionID string, input SnapshotInput) (SnapshotResult, error) {
	path := sessionPath(runtimeSessionID, "snapshot")
	if strings.TrimSpace(input.Mode) != "" {
		path += "?mode=" + url.QueryEscape(strings.TrimSpace(input.Mode))
	}
	var out SnapshotResult
	err := c.doJSON(ctx, http.MethodGet, path, nil, &out)
	return out, err
}

func (c *Client) Act(ctx context.Context, runtimeSessionID string, input ActInput) (ActResult, error) {
	if err := ValidateActInput(input); err != nil {
		return ActResult{}, err
	}
	var out ActResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "act"), input, &out)
	return out, err
}

func (c *Client) WaitFor(ctx context.Context, runtimeSessionID string, input WaitForInput) (WaitForResult, error) {
	var out WaitForResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "wait-for"), input, &out)
	return out, err
}

func (c *Client) PageTool(ctx context.Context, runtimeSessionID string, input PageToolInput) (PageToolResult, error) {
	if err := ValidatePageToolInput(input); err != nil {
		return nil, err
	}
	var out PageToolResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "pageTool"), input, &out)
	return out, err
}

func (c *Client) Evaluate(ctx context.Context, runtimeSessionID string, input EvaluateInput) (EvaluateResult, error) {
	var out EvaluateResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "evaluate"), input, &out)
	return out, err
}

func (c *Client) Screenshot(ctx context.Context, runtimeSessionID string, input ScreenshotInput) (ScreenshotResult, error) {
	if err := ValidateScreenshotInput(input); err != nil {
		return ScreenshotResult{}, err
	}
	var out ScreenshotResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "screenshot"), input, &out)
	return out, err
}

func (c *Client) UploadFiles(ctx context.Context, runtimeSessionID string, input UploadFilesInput) (UploadFilesResult, error) {
	var out UploadFilesResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "upload-files"), input, &out)
	return out, err
}

func (c *Client) LiveView(ctx context.Context, runtimeSessionID string, input LiveViewInput) (LiveViewResult, error) {
	var out LiveViewResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "live-view"), input, &out)
	return out, err
}

func (c *Client) StartHandoff(ctx context.Context, runtimeSessionID string, input StartHandoffInput) (HandoffResult, error) {
	var out HandoffResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "handoff/start"), input, &out)
	return out, err
}

func (c *Client) CompleteHandoff(ctx context.Context, runtimeSessionID string, handoffID string) error {
	return c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "handoff/"+url.PathEscape(strings.TrimSpace(handoffID))+"/complete"), nil, nil)
}

func (c *Client) Commit(ctx context.Context, runtimeSessionID string, input CommitInput) (CommitResult, error) {
	var out CommitResult
	err := c.doJSON(ctx, http.MethodPost, sessionPath(runtimeSessionID, "commit"), input, &out)
	return out, err
}

func (c *Client) Close(ctx context.Context, runtimeSessionID string) error {
	return c.doJSON(ctx, http.MethodDelete, "/v1/sessions/"+url.PathEscape(strings.TrimSpace(runtimeSessionID)), nil, nil)
}

func (c *Client) doJSON(ctx context.Context, method string, path string, input any, output any) error {
	if c == nil || strings.TrimSpace(c.baseURL) == "" || c.httpClient == nil {
		return Error{Code: "browserd_client_not_configured", Message: "browserd client is not configured", Operation: method, Path: path}
	}
	var body io.Reader
	if input != nil {
		raw, err := json.Marshal(input)
		if err != nil {
			return Error{Code: "browserd_request_invalid", Message: err.Error(), Operation: method, Path: path, Cause: err}
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return Error{Code: "browserd_request_invalid", Message: err.Error(), Operation: method, Path: path, Cause: err}
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return Error{Code: "browserd_unavailable", Message: err.Error(), Operation: method, Path: path, Cause: err}
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return Error{Code: "browserd_response_invalid", Message: err.Error(), StatusCode: res.StatusCode, Operation: method, Path: path, Cause: err}
	}
	var envelope responseEnvelope[json.RawMessage]
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Error{Code: "browserd_response_invalid", Message: err.Error(), StatusCode: res.StatusCode, Operation: method, Path: path, Cause: err}
	}
	if envelope.Error != nil {
		return Error{Code: envelope.Error.Code, Message: envelope.Error.Message, StatusCode: res.StatusCode, Operation: method, Path: path}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Error{Code: "browserd_request_failed", Message: fmt.Sprintf("browserd returned status %d", res.StatusCode), StatusCode: res.StatusCode, Operation: method, Path: path}
	}
	if envelope.Data == nil {
		return Error{Code: "browserd_response_invalid", Message: "missing data", StatusCode: res.StatusCode, Operation: method, Path: path}
	}
	if output == nil {
		return nil
	}
	if err := json.Unmarshal(*envelope.Data, output); err != nil {
		return Error{Code: "browserd_response_invalid", Message: err.Error(), StatusCode: res.StatusCode, Operation: method, Path: path, Cause: err}
	}
	return nil
}

func sessionPath(runtimeSessionID string, action string) string {
	return "/v1/sessions/" + url.PathEscape(strings.TrimSpace(runtimeSessionID)) + "/" + action
}
