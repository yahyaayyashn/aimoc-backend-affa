package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPruneOldUsesRecordingTimestampNotMtime — regresi utk bug 04 Agu 2026: pruneOld
// dulu mengukur umur file dari mtime (waktu download), bukan waktu rekam asli di nama
// file. Skenario di sini SENGAJA meniru bug itu: kedua file baru saja "disentuh" (mtime
// sama, "sekarang") padahal salah satu namanya merekam waktu 5 hari lalu -- kalau
// pruneOld masih pakai mtime, file lama itu TIDAK akan terhapus (bug), padahal
// seharusnya terhapus karena rekamannya sudah lewat maxAge.
func TestPruneOldUsesRecordingTimestampNotMtime(t *testing.T) {
	dir := t.TempDir()
	camDir := filepath.Join(dir, "CAM-TEST")
	if err := os.MkdirAll(camDir, 0755); err != nil {
		t.Fatal(err)
	}

	oldName := time.Now().Add(-5 * 24 * time.Hour).Format("20060102_150405") + "_NF.mp4"
	newName := time.Now().Add(-1 * time.Hour).Format("20060102_150405") + "_NF.mp4"
	for _, name := range []string{oldName, newName} {
		if err := os.WriteFile(filepath.Join(camDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	// Samakan mtime kedua file ke "sekarang" -- meniru kondisi nyata bug: file
	// ke-download/tersentuh belakangan meski rekaman aslinya sudah lama.
	now := time.Now()
	for _, name := range []string{oldName, newName} {
		if err := os.Chtimes(filepath.Join(camDir, name), now, now); err != nil {
			t.Fatal(err)
		}
	}

	s := &VODSyncService{RecordingsDir: dir}
	s.pruneOld("CAM-TEST", 72*time.Hour)

	if _, err := os.Stat(filepath.Join(camDir, oldName)); !os.IsNotExist(err) {
		t.Errorf("file rekaman 5 hari lalu (%s) seharusnya terhapus (retensi 72 jam berbasis waktu rekam), tapi masih ada", oldName)
	}
	if _, err := os.Stat(filepath.Join(camDir, newName)); err != nil {
		t.Errorf("file rekaman 1 jam lalu (%s) seharusnya TIDAK terhapus, tapi hilang: %v", newName, err)
	}
}
