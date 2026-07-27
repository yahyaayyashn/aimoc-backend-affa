package service

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/pkg/blackvue"
	"aimoc-backend/pkg/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// VODSyncService — worker latar belakang yang menarik segmen rekaman VOD dari tiap
// dashcam BlackVue (via Fleet Server) ke folder lokal RecordingsDir/<camera_code>/.
// Folder ini dibaca AI service sebagai sumber FAIL-SAFE saat live feed putus (auto-
// failover live→recording). Reuse pkg/blackvue (ListFolder/DownloadFile/HourFolders).
//
// Idempoten: segmen yang sudah ada di folder tidak diunduh ulang. Atomik: DownloadFile
// menulis ke .part lalu rename. Serialisasi per device fisik (mac) supaya tidak ada 2
// unduhan paralel ke dashcam yang sama (kapasitas tunnel FRP terbatas).
type VODSyncService struct {
	DB           *gorm.DB
	RecordingsDir string
	IntervalSec  int
	stop         chan struct{}
	locks        sync.Map // mac -> *sync.Mutex
}

func NewVODSyncService(db *gorm.DB, recordingsDir string, intervalSec int) *VODSyncService {
	return &VODSyncService{DB: db, RecordingsDir: recordingsDir, IntervalSec: intervalSec, stop: make(chan struct{})}
}

// Start menjalankan loop sync di goroutine terpisah. Poll pertama langsung, lalu tiap
// IntervalSec. Panggil sekali dari main.
func (s *VODSyncService) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil && utils.Log != nil {
				utils.Log.Error("vod sync loop panic", zap.Any("recover", r))
			}
		}()
		s.syncAll()
		ticker := time.NewTicker(time.Duration(s.IntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-ticker.C:
				s.syncAll()
			}
		}
	}()
	if utils.Log != nil {
		utils.Log.Info("VOD sync worker aktif", zap.Int("interval_sec", s.IntervalSec), zap.String("dir", s.RecordingsDir))
	}
}

func (s *VODSyncService) Stop() { close(s.stop) }

// syncAll memproses semua kamera BlackVue aktif.
func (s *VODSyncService) syncAll() {
	var cams []domain.Camera
	if err := s.DB.Where("status = ? AND stream_url LIKE ?", "AKTIF", "blackvue://%").Find(&cams).Error; err != nil {
		if utils.Log != nil {
			utils.Log.Warn("vod sync: gagal ambil daftar kamera", zap.Error(err))
		}
		return
	}
	for _, cam := range cams {
		s.syncCamera(cam)
	}
}

// syncCamera menarik segmen Front baru dari 1 dashcam untuk ~2 jam terakhir.
func (s *VODSyncService) syncCamera(cam domain.Camera) {
	mac, psn, fleetHost, fleetAPIPort, _, err := blackvue.ParseStreamURL(cam.StreamURL)
	if err != nil {
		return // bukan blackvue:// valid — lewati diam-diam
	}

	muRaw, _ := s.locks.LoadOrStore(mac, &sync.Mutex{})
	mu := muRaw.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	port, err := blackvue.ResolveCGIPort(fleetHost, fleetAPIPort, mac, psn)
	if err != nil {
		if utils.Log != nil {
			utils.Log.Warn("vod sync: gagal resolve CGI port", zap.String("camera", cam.Code), zap.Error(err))
		}
		return
	}

	dir := filepath.Join(s.RecordingsDir, cam.Code)
	if err := os.MkdirAll(dir, 0755); err != nil {
		if utils.Log != nil {
			utils.Log.Error("vod sync: gagal buat folder", zap.String("dir", dir), zap.Error(err))
		}
		return
	}

	now := time.Now()
	windowStart := now.Add(-2 * time.Hour)
	seen := map[string]bool{}
	fetched := 0
	for _, folder := range blackvue.HourFolders(windowStart, now) {
		entries, err := blackvue.ListFolder(fleetHost, port, folder)
		if err != nil {
			continue // folder berikutnya
		}
		for _, e := range entries {
			if e.Camera != 'F' || seen[e.Filename] {
				continue
			}
			seen[e.Filename] = true
			dest := filepath.Join(dir, e.Filename)
			if _, statErr := os.Stat(dest); statErr == nil {
				continue // sudah ada — idempoten
			}
			if _, err := blackvue.DownloadFile(fleetHost, port, folder, e.Filename, dest); err != nil {
				if utils.Log != nil {
					utils.Log.Warn("vod sync: gagal download segmen",
						zap.String("camera", cam.Code), zap.String("file", e.Filename), zap.Error(err))
				}
				continue
			}
			fetched++
		}
	}
	if fetched > 0 && utils.Log != nil {
		utils.Log.Info("vod sync: segmen baru tersimpan",
			zap.String("camera", cam.Code), zap.Int("count", fetched))
	}

	// Retensi sederhana: buang file rekaman lebih tua dari 24 jam supaya folder tidak
	// membengkak (fail-safe hanya butuh rekaman terbaru).
	s.pruneOld(dir, 24*time.Hour)
}

// pruneOld menghapus file .mp4 di dir yang lebih tua dari maxAge (berbasis mtime).
func (s *VODSyncService) pruneOld(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".mp4") {
			continue
		}
		info, err := ent.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, ent.Name()))
		}
	}
}
