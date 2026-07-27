package service

import (
	"time"

	"aimoc-backend/internal/domain"
	"aimoc-backend/pkg/blackvue"
	"aimoc-backend/pkg/utils"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// sdCardFullRatio -- SD card dianggap FULL kalau avail/total di bawah rasio ini
// (bukan cuma "avail==0" persis, sesuai definisi "mendekati 0" di daftar gangguan).
const sdCardFullRatio = 0.05

const socketQueryTimeout = 8 * time.Second

// DeviceHealthPoller — worker latar belakang yang query command-socket dashcam (SD
// card/RTC/GPS/SIM, lihat pkg/blackvue/socket.go) tiap IntervalSec dan simpan hasilnya
// ke kolom cameras.* (migration 000025). Dipakai halaman "02 - Detail Excavator" dan
// "03 - Pemeriksaan Unit" (lewat CameraIncidentService.GetOrCreateActive yang membaca
// kolom ini juga). Pola sama persis VODSyncService: poll pertama langsung, lalu ticker
// interval, error 1 kamera tidak menghentikan yang lain.
type DeviceHealthPoller struct {
	DB          *gorm.DB
	IntervalSec int
	stop        chan struct{}
}

func NewDeviceHealthPoller(db *gorm.DB, intervalSec int) *DeviceHealthPoller {
	return &DeviceHealthPoller{DB: db, IntervalSec: intervalSec, stop: make(chan struct{})}
}

func (p *DeviceHealthPoller) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil && utils.Log != nil {
				utils.Log.Error("device health poller panic", zap.Any("recover", r))
			}
		}()
		p.pollAll()
		ticker := time.NewTicker(time.Duration(p.IntervalSec) * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-ticker.C:
				p.pollAll()
			}
		}
	}()
	if utils.Log != nil {
		utils.Log.Info("Device health poller aktif", zap.Int("interval_sec", p.IntervalSec))
	}
}

func (p *DeviceHealthPoller) Stop() { close(p.stop) }

func (p *DeviceHealthPoller) pollAll() {
	var cams []domain.Camera
	if err := p.DB.Where("status = ? AND stream_url LIKE ?", "AKTIF", "blackvue://%").Find(&cams).Error; err != nil {
		if utils.Log != nil {
			utils.Log.Warn("device health: gagal ambil daftar kamera", zap.Error(err))
		}
		return
	}
	for _, cam := range cams {
		p.pollCamera(cam)
	}
}

// pollCamera membuka 1 koneksi command-socket, query SD card/RTC/GPS/SIM berurutan
// (idle timeout command-socket ~15s -- 1 koneksi per siklus, bukan koneksi persisten),
// lalu simpan semua hasil dalam 1 UPDATE. Kegagalan 1 command tidak menggagalkan yang
// lain -- kolom yang gagal dibaca cukup dibiarkan tidak diupdate siklus ini.
func (p *DeviceHealthPoller) pollCamera(cam domain.Camera) {
	mac, psn, fleetHost, fleetAPIPort, _, err := blackvue.ParseStreamURL(cam.StreamURL)
	if err != nil {
		return // bukan blackvue:// valid -- lewati diam-diam
	}

	port, err := blackvue.ResolveSocketPort(fleetHost, fleetAPIPort, mac, psn)
	if err != nil {
		if utils.Log != nil {
			utils.Log.Warn("device health: gagal resolve socket port", zap.String("camera", cam.Code), zap.Error(err))
		}
		return
	}

	client, err := blackvue.DialSocket(fleetHost, port, socketQueryTimeout)
	if err != nil {
		if utils.Log != nil {
			utils.Log.Warn("device health: gagal dial command-socket", zap.String("camera", cam.Code), zap.Error(err))
		}
		return
	}
	defer client.Close()

	updates := map[string]interface{}{"health_checked_at": time.Now()}

	if sd, err := client.QuerySDCardInfo(socketQueryTimeout); err == nil && sd != nil {
		updates["sd_status"] = sdStatusLabel(sd)
		updates["sd_avail_kb"] = sd.Avail
	} else {
		// Request gagal/timeout untuk 0x0003 == "SD card tidak terdeteksi" (sesuai
		// definisi di daftar gangguan, bukan cuma error jaringan biasa).
		updates["sd_status"] = "NOT_DETECTED"
		if utils.Log != nil {
			utils.Log.Debug("device health: query SD card gagal", zap.String("camera", cam.Code), zap.Error(err))
		}
	}

	if rtc, err := client.QueryRTCStatus(socketQueryTimeout); err == nil && rtc != nil {
		updates["rtc_low_battery"] = rtc.LowVol == 1
	}

	if drv, err := client.QueryDrivingInfo(socketQueryTimeout); err == nil && drv != nil {
		if drv.NoGPSDuration > 0 {
			if cam.GPSNoSignalSince == nil {
				now := time.Now()
				updates["gps_no_signal_since"] = &now
			}
			// kalau sudah ada gps_no_signal_since sebelumnya, biarkan (menandai SEJAK
			// kapan masalah ini terus berlangsung, bukan direset tiap poll).
		} else {
			updates["gps_no_signal_since"] = nil // sinyal pulih
		}
	}

	if sim, err := client.QuerySIMStatus(socketQueryTimeout); err == nil && sim != nil {
		updates["sim_status"] = simStatusLabel(sim)
	}

	if err := p.DB.Model(&domain.Camera{}).Where("id = ?", cam.ID).Updates(updates).Error; err != nil {
		if utils.Log != nil {
			utils.Log.Warn("device health: gagal simpan hasil", zap.String("camera", cam.Code), zap.Error(err))
		}
	}
}

func sdStatusLabel(sd *blackvue.SDCardInfo) string {
	if sd.Total > 0 && float64(sd.Avail)/float64(sd.Total) < sdCardFullRatio {
		return "FULL"
	}
	switch sd.Status {
	case 0x01:
		return "WRITE_FAILURE"
	case 0x02:
		return "WRITE_PROTECTED"
	case 0x04:
		return "RW_FAILURE"
	default:
		return "NORMAL"
	}
}

func simStatusLabel(sim *blackvue.SIMStatus) string {
	if sim.PwrStatus == 0 {
		return "MODEM_OFF"
	}
	if sim.ErrCode != 0 {
		return "ERROR"
	}
	return "NORMAL"
}
