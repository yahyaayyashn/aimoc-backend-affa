package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/pkg/blackvue"
	"aimoc-backend/pkg/utils"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CameraStreamHandler struct {
	DB                *gorm.DB
	TestVideosDir     string
	AIServiceDebugURL string
	RecordingsDir     string
}

func NewCameraStreamHandler(db *gorm.DB, testVideosDir, aiServiceDebugURL, recordingsDir string) *CameraStreamHandler {
	return &CameraStreamHandler{DB: db, TestVideosDir: testVideosDir, AIServiceDebugURL: aiServiceDebugURL, RecordingsDir: recordingsDir}
}

// resolveVideoFilePath ekstrak nama file dari skema video://<nama-file>, resolve ke path
// penuh di dalam TestVideosDir, dan tolak kalau hasilnya keluar dari direktori itu (path
// traversal guard) -- pola sama seperti VideoFileCapture di aimoc-ai-service supaya kedua
// sisi konsisten.
func resolveVideoFilePath(baseDir, streamURL string) (string, error) {
	rel := strings.TrimPrefix(streamURL, "video://")
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("path video kosong")
	}
	base := filepath.Clean(baseDir)
	full := filepath.Clean(filepath.Join(base, rel))
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path video di luar direktori yang diizinkan")
	}
	if _, err := os.Stat(full); err != nil {
		return "", fmt.Errorf("file video tidak ditemukan: %s", rel)
	}
	return full, nil
}

// Live menyajikan live-view kamera -- dua skema didukung:
//   - video://<nama-file>: kamera "palsu" pakai video file lokal sebagai pengganti dashcam
//     (testing/simulasi, lihat rencana "Sumber kamera per-excavator"), diserve langsung lewat
//     c.SendFile (otomatis dukung Range header untuk seeking <video>, tidak perlu proxy manual).
//   - blackvue://<mac>:<psn>@<fleet_host>:<fleet_api_port>/<live|hss>: proxy MJPEG dashcam asli
//     (multipart/x-mixed-replace, didukung native oleh tag <img>) -- sama seperti CAMERAS di
//     aimoc-ai-service, satu konvensi URL dipakai di kedua sisi.
func (h *CameraStreamHandler) Live(c *fiber.Ctx) error {
	var cam domain.Camera
	if err := h.DB.First(&cam, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Kamera tidak ditemukan")
	}

	if strings.HasPrefix(cam.StreamURL, "video://") {
		path, err := resolveVideoFilePath(h.TestVideosDir, cam.StreamURL)
		if err != nil {
			return utils.NotFound(c, err.Error())
		}
		return c.SendFile(path)
	}

	mac, psn, fleetHost, fleetAPIPort, mode, err := blackvue.ParseStreamURL(cam.StreamURL)
	if err != nil {
		return utils.BadRequest(c, "Kamera ini belum dikonfigurasi untuk live stream BlackVue (stream_url harus berskema blackvue://mac:psn@host:port/live): "+err.Error(), nil)
	}

	port, err := blackvue.ResolveCGIPort(fleetHost, fleetAPIPort, mac, psn)
	if err != nil {
		return utils.ServerError(c, "Gagal konek ke dashcam: "+err.Error(), nil)
	}

	cgiFile := "blackvue_live.cgi"
	if mode == "hss" {
		cgiFile = "blackvue_hss_live.cgi"
	}
	targetURL := fmt.Sprintf("http://%s:%d/%s", fleetHost, port, cgiFile)

	upstreamResp, err := http.Get(targetURL) // tanpa timeout keseluruhan — stream MJPEG hidup terus
	if err != nil {
		return utils.ServerError(c, "Gagal konek ke live stream dashcam: "+err.Error(), nil)
	}
	if upstreamResp.StatusCode != http.StatusOK {
		upstreamResp.Body.Close()
		return utils.ServerError(c, fmt.Sprintf("Dashcam merespon HTTP %d", upstreamResp.StatusCode), nil)
	}

	ct := upstreamResp.Header.Get("Content-Type")
	if ct == "" {
		ct = "multipart/x-mixed-replace"
	}
	c.Set("Content-Type", ct)
	c.Set("Cache-Control", "no-store")
	// fasthttp menutup upstreamResp.Body (implements io.Closer) otomatis setelah selesai/klien putus.
	c.Context().Response.SetBodyStream(upstreamResp.Body, -1)
	return nil
}

// RestartVideo me-relay permintaan "putar ulang video dari awal" ke debug listener
// internal aimoc-ai-service (lihat debug_listener.py, POST /debug/camera/<code>/restart)
// -- endpoint itu SENGAJA tidak dipublish ke host/browser, jadi backend yang jadi
// jembatan satu-satunya buat tombol restart di FE. Hanya berlaku untuk kamera video://
// (dashcam asli tidak punya konsep "restart", live stream-nya memang terus-menerus).
//
// PENTING: relay ini pakai endpoint /restart, BUKAN /video-file -- /video-file akan
// mengubah _current_mode pipeline jadi "file" (debug-override) SECARA PERMANEN,
// merusak properti "persisten ikut Camera.stream_url" milik skema video:// (kamera
// jadi terkunci ke mode debug walau cuma mau "putar ulang"). /restart cuma membuka
// ulang capture SAAT INI tanpa mengubah mode sama sekali.
func (h *CameraStreamHandler) RestartVideo(c *fiber.Ctx) error {
	var cam domain.Camera
	if err := h.DB.First(&cam, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Kamera tidak ditemukan")
	}
	if !strings.HasPrefix(cam.StreamURL, "video://") {
		return utils.BadRequest(c, "Kamera ini tidak dikonfigurasi mode video, tidak ada yang perlu di-restart", nil)
	}

	target := fmt.Sprintf("%s/debug/camera/%s/restart", strings.TrimRight(h.AIServiceDebugURL, "/"), url.PathEscape(cam.Code))
	resp, err := http.Post(target, "application/json", nil)
	if err != nil {
		return utils.ServerError(c, "Gagal menghubungi AI service: "+err.Error(), nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return utils.ServerError(c, fmt.Sprintf("AI service merespon HTTP %d", resp.StatusCode), nil)
	}
	return utils.OK(c, "Video di-restart dari awal", nil)
}

// VideoStatus — relay posisi baca AI service untuk kamera mode video:// (jam mulai
// baca video + jam server AI). Dipakai LiveCameraFeed di FE untuk memutar live-view
// dari POSISI YANG SAMA dengan yang sedang dianalisis AI (bukan selalu dari detik 0),
// supaya tampilan live bisa disandingkan dengan hitungan kerukan yang berjalan.
// Pola relay sama dengan RestartVideo di atas: browser tidak pernah menjangkau
// debug listener :8090 langsung.
func (h *CameraStreamHandler) VideoStatus(c *fiber.Ctx) error {
	var cam domain.Camera
	if err := h.DB.First(&cam, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Kamera tidak ditemukan")
	}
	if !strings.HasPrefix(cam.StreamURL, "video://") {
		return utils.BadRequest(c, "Kamera ini tidak dikonfigurasi mode video", nil)
	}

	target := fmt.Sprintf("%s/debug/camera/%s/status", strings.TrimRight(h.AIServiceDebugURL, "/"), url.PathEscape(cam.Code))
	resp, err := http.Get(target)
	if err != nil {
		return utils.ServerError(c, "Gagal menghubungi AI service: "+err.Error(), nil)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return utils.ServerError(c, fmt.Sprintf("AI service merespon HTTP %d", resp.StatusCode), nil)
	}
	var payload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return utils.ServerError(c, "Respon AI service tidak valid: "+err.Error(), nil)
	}
	return utils.OK(c, "OK", payload)
}

// recordingFilenameRe menangkap konvensi penamaan BlackVue: <YYYYMMDD>_<HHMMSS>_<TYPE><CAM>.mp4
// -- sama seperti pkg/blackvue.filenameRe, diduplikasi ringan di sini (bukan di-export)
// karena RecordingsDir dibaca langsung dari disk lokal (hasil VODSyncService), bukan
// lewat blackvue_vod.cgi seperti pkg/blackvue.ListFolder.
var recordingFilenameRe = regexp.MustCompile(`^(\d{8})_(\d{6})_([NPEM])([FR])\.mp4$`)

type RecordingEntry struct {
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
	StartTS   string `json:"start_ts"` // ISO8601, jam device lokal (Asia/Jakarta)
}

// resolveRecordingPath resolve nama file rekaman ke path penuh di dalam
// RecordingsDir/<camera_code>/, dengan path-traversal guard -- pola sama seperti
// resolveVideoFilePath di atas.
func resolveRecordingPath(recordingsDir, cameraCode, filename string) (string, error) {
	if strings.TrimSpace(filename) == "" || strings.ContainsAny(filename, "/\\") {
		return "", fmt.Errorf("nama file tidak valid")
	}
	base := filepath.Clean(filepath.Join(recordingsDir, cameraCode))
	full := filepath.Clean(filepath.Join(base, filename))
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", fmt.Errorf("path di luar direktori yang diizinkan")
	}
	if _, err := os.Stat(full); err != nil {
		return "", fmt.Errorf("file rekaman tidak ditemukan: %s", filename)
	}
	return full, nil
}

// Recordings — daftar klip VOD historis kamera ini untuk 1 tanggal (fail-safe rekaman
// yang disinkronkan VODSyncService ke RecordingsDir/<camera_code>/*.mp4). Dipakai menu
// "Kamera & Rekaman" tab Riwayat: user pilih tanggal, lihat daftar klip untuk diputar
// atau diunduh -- TANPA autoplay, murni klik-kontrol user (lihat RecordingFile).
func (h *CameraStreamHandler) Recordings(c *fiber.Ctx) error {
	var cam domain.Camera
	if err := h.DB.First(&cam, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Kamera tidak ditemukan")
	}
	date := c.Query("date", "") // YYYY-MM-DD
	if date == "" {
		return utils.BadRequest(c, "Parameter date wajib diisi (format YYYY-MM-DD)", nil)
	}

	dir := filepath.Join(h.RecordingsDir, cam.Code)
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Folder belum ada (belum pernah sync) bukan error -- cukup daftar kosong.
		return utils.OK(c, "OK", []RecordingEntry{})
	}

	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		jakarta = time.FixedZone("WIB", 7*60*60)
	}

	var rows []RecordingEntry
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		m := recordingFilenameRe.FindStringSubmatch(ent.Name())
		if m == nil || m[4][0] != 'F' { // hanya kamera Front, konsisten dengan ClipBackup/VODSync lama
			continue
		}
		ts, err := time.ParseInLocation("20060102150405", m[1]+m[2], jakarta)
		if err != nil {
			continue
		}
		if ts.Format("2006-01-02") != date {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		rows = append(rows, RecordingEntry{
			Filename:  ent.Name(),
			SizeBytes: info.Size(),
			StartTS:   ts.Format(time.RFC3339),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].StartTS < rows[j].StartTS })
	if rows == nil {
		rows = []RecordingEntry{}
	}
	return utils.OK(c, "OK", rows)
}

// RecordingFile menyajikan 1 file rekaman historis spesifik by name -- beda dari
// Live() yang selalu "sumber saat ini". Diserve lewat c.SendFile (dukung Range header
// untuk seek/download parsial), preload="none" di FE supaya TIDAK ada request sampai
// user eksplisit klik putar/download (lihat rencana "Kamera & Rekaman").
func (h *CameraStreamHandler) RecordingFile(c *fiber.Ctx) error {
	var cam domain.Camera
	if err := h.DB.First(&cam, "id = ?", c.Params("id")).Error; err != nil {
		return utils.NotFound(c, "Kamera tidak ditemukan")
	}
	path, err := resolveRecordingPath(h.RecordingsDir, cam.Code, c.Params("filename"))
	if err != nil {
		return utils.NotFound(c, err.Error())
	}
	return c.SendFile(path)
}
