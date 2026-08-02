// Package aivision berisi client HTTP tipis untuk service Python eksternal tim AI
// (excavator_vlm/aimoc_excavator_pipeline, Gracia BCS): YOLO+BoT-SORT+LSTM asli yang
// mendeteksi truk secara visual dari video, dijalankan sebagai job upload+async GPU
// queue (bukan live-stream). Lihat service/DOCUMENTATION.md di repo mereka untuk skema
// endpoint lengkap. Dipakai internal/service/ai_vision.go.
package aivision

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 5 * time.Minute}, // upload video besar butuh waktu
	}
}

func (c *Client) do(req *http.Request) (*http.Response, error) {
	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	}
	return c.HTTP.Do(req)
}

// UploadVideoResp — respons POST /api/videos.
type UploadVideoResp struct {
	VideoID  string `json:"video_id"`
	Filename string `json:"filename"`
	SizeByte int64  `json:"size_bytes"`
}

// UploadVideo mengirim file video lokal ke POST /api/videos (multipart), balik
// video_id untuk dipakai SubmitJob.
func (c *Client) UploadVideo(path string) (*UploadVideoResp, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("gagal buka file video: %w", err)
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("gagal siapkan multipart: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, fmt.Errorf("gagal salin isi video ke multipart: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gagal tutup multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/videos", &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal upload video ke AI vision service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("upload video HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out UploadVideoResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gagal parse respons upload video: %w", err)
	}
	return &out, nil
}

// SubmitJob POST /api/jobs — masukkan video (dari UploadVideo) ke antrean GPU. recordedAt
// format RFC3339, boleh kosong (service isi otomatis dengan waktu sekarang).
func (c *Client) SubmitJob(videoID, originalFilename, unitID, recordedAt string) (*Job, error) {
	payload, _ := json.Marshal(map[string]string{
		"video_id":          videoID,
		"original_filename": originalFilename,
		"unit_id":           unitID,
		"recorded_at":       recordedAt,
	})
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+"/api/jobs", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal submit job ke AI vision service: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("submit job HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out Job
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gagal parse respons submit job: %w", err)
	}
	return &out, nil
}

// Job — bentuk minimal respons GET/POST /api/jobs(/:id) yang benar-benar dipakai
// (lihat service/main.py's _public_job -- field lain di respons asli diabaikan,
// dashboard_summary disimpan mentah sebagai json.RawMessage).
type Job struct {
	JobID        string            `json:"job_id"`
	Status       string            `json:"status"`
	Stage        string            `json:"stage"`
	Progress     int               `json:"progress"`
	Message      string            `json:"message"`
	Error        string            `json:"error"`
	Dashboard    json.RawMessage   `json:"dashboard"`
	ArtifactURLs map[string]string `json:"artifact_urls"`
}

// GetJob GET /api/jobs/{id} — dipoll berkala sampai status completed/failed/cancelled.
func (c *Client) GetJob(jobID string) (*Job, error) {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+"/api/jobs/"+jobID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal poll job AI vision: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("poll job HTTP %d: %s", resp.StatusCode, string(b))
	}
	var out Job
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gagal parse respons poll job: %w", err)
	}
	return &out, nil
}

// ArtifactURL — URL lengkap ke 1 artifact job (video ber-anotasi, dsb), dipakai backend
// untuk proxy stream (browser tidak pernah panggil service Python langsung -- API key
// harus tetap di sisi backend).
func (c *Client) ArtifactURL(jobID, key string) string {
	return fmt.Sprintf("%s/api/jobs/%s/artifacts/%s", c.BaseURL, jobID, key)
}

// OpenArtifact — buka stream 1 artifact untuk diproxy langsung (caller wajib Close()
// body-nya).
func (c *Client) OpenArtifact(jobID, key string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.ArtifactURL(jobID, key), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("gagal ambil artifact AI vision: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifact HTTP %d: %s", resp.StatusCode, string(b))
	}
	return resp, nil
}
