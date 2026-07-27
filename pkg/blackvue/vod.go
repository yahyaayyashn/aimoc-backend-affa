package blackvue

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// jakarta adalah lokasi waktu yang dipakai untuk menafsirkan timestamp pada nama file
// dashcam (jam device lokal, bukan UTC) dan untuk menghitung nama folder d:/Record/.
// Di-hardcode (bukan ambient time.Local) supaya tidak bergantung pada timezone OS
// server tempat backend jalan — konsisten dengan APP_TIMEZONE=Asia/Jakarta yang
// dipakai project ini di tempat lain (internal/config/config.go).
var jakarta = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}()

type VODEntry struct {
	Filename string
	SizeByte int64
	TS       time.Time // waktu mulai segmen, jam device lokal (Asia/Jakarta)
	RecType  byte       // 'N'=Normal | 'P'=Parking | 'E'=Event | 'M'=Manual
	Camera   byte       // 'F'=Front | 'R'=Rear
}

// filenameRe menangkap konvensi penamaan BlackVue: <YYYYMMDD>_<HHMMSS>_<TYPE><CAM>.mp4
var filenameRe = regexp.MustCompile(`(\d{8})_(\d{6})_([NPEM])([FR])\.mp4`)

// sizeRe menangkap ,s:<angka> yang menyertai tiap baris listing (ukuran dalam byte).
var sizeRe = regexp.MustCompile(`,s:(\d+)`)

// ListFolder GET blackvue_vod.cgi?path=d:/Record/<folder>/ dan parse hasilnya.
// TIDAK mempercayai pemisahan "n:name,s:size" yang bersih — ada bug tampilan
// terdokumentasi di Fleet Server dimana path folder & nama file tergabung tanpa
// separator (mis. "n:/Record/20260709_1920260709_191331_PF.mp4"). Karena itu tiap
// baris di-regex-scan mencari pola nama file yang valid di posisi manapun, bukan
// split by "," naif.
func ListFolder(fleetHost string, cgiPort int, folder string) ([]VODEntry, error) {
	listURL := fmt.Sprintf("http://%s:%d/blackvue_vod.cgi?path=d:/Record/%s/", fleetHost, cgiPort, folder)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(listURL)
	if err != nil {
		return nil, fmt.Errorf("gagal list folder %s: %w", folder, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list folder %s: HTTP %d", folder, resp.StatusCode)
	}

	var entries []VODEntry
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "n:") {
			continue // lewati baris folder ("d:") dan header ("v:")
		}
		m := filenameRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ts, err := time.ParseInLocation("20060102150405", m[1]+m[2], jakarta)
		if err != nil {
			continue
		}
		var size int64
		if sm := sizeRe.FindStringSubmatch(line); sm != nil {
			size, _ = strconv.ParseInt(sm[1], 10, 64)
		}
		entries = append(entries, VODEntry{
			Filename: m[0],
			SizeByte: size,
			TS:       ts,
			RecType:  m[3][0],
			Camera:   m[4][0],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("gagal baca response list folder %s: %w", folder, err)
	}
	return entries, nil
}

// HourFolders menghitung semua folder "YYYYMMDD_HH" (jam device lokal, Asia/Jakarta)
// yang meng-cover rentang [from, to], inklusif, termasuk lintas jam/hari, tanpa duplikat.
func HourFolders(from, to time.Time) []string {
	from, to = from.In(jakarta), to.In(jakarta)
	if to.Before(from) {
		from, to = to, from
	}
	var folders []string
	seen := map[string]bool{}
	cur := from.Truncate(time.Hour)
	for !cur.After(to) {
		f := cur.Format("20060102_15")
		if !seen[f] {
			seen[f] = true
			folders = append(folders, f)
		}
		cur = cur.Add(time.Hour)
	}
	return folders
}

// DownloadFile GET http://<host>:<port>/Record/<folder>/<filename> dan tulis ke destPath
// secara atomik (tulis ke destPath+".part" dulu, baru rename). Timeout 10 menit di sini
// adalah stall-guard, bukan throughput-budget — transfer lambat-tapi-aktif via tunnel FRP
// terbukti TIDAK di-drop (cuma koneksi idle yang di-drop), jadi client dibuat sabar.
func DownloadFile(fleetHost string, cgiPort int, folder, filename, destPath string) (int64, error) {
	fileURL := fmt.Sprintf("http://%s:%d/Record/%s/%s", fleetHost, cgiPort, folder, filename)
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(fileURL)
	if err != nil {
		return 0, fmt.Errorf("gagal download %s: %w", filename, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download %s: HTTP %d", filename, resp.StatusCode)
	}

	partPath := destPath + ".part"
	out, err := os.Create(partPath)
	if err != nil {
		return 0, fmt.Errorf("gagal buat file tujuan: %w", err)
	}
	n, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(partPath)
		return 0, fmt.Errorf("gagal tulis %s: %w", filename, copyErr)
	}
	if closeErr != nil {
		os.Remove(partPath)
		return 0, fmt.Errorf("gagal tutup file %s: %w", filename, closeErr)
	}
	if err := os.Rename(partPath, destPath); err != nil {
		os.Remove(partPath)
		return 0, fmt.Errorf("gagal rename file %s: %w", filename, err)
	}
	return n, nil
}
