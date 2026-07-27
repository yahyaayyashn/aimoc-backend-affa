-- Migration: 000025_camera_device_health
-- Kolom hasil polling command-socket dashcam (SD card/RTC/GPS/SIM) -- lihat
-- pkg/blackvue/socket.go + internal/service/device_health.go. Pola sama seperti
-- last_activity_status/last_seen_at yang sudah ada di tabel ini.

ALTER TABLE cameras
    ADD COLUMN IF NOT EXISTS sd_status          varchar(30) NULL,
    ADD COLUMN IF NOT EXISTS sd_avail_kb         bigint NULL,
    ADD COLUMN IF NOT EXISTS gps_no_signal_since timestamptz NULL,
    ADD COLUMN IF NOT EXISTS rtc_low_battery     boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS sim_status          varchar(30) NULL,
    ADD COLUMN IF NOT EXISTS health_checked_at   timestamptz NULL;

COMMENT ON COLUMN cameras.sd_status IS 'NORMAL | WRITE_FAILURE | WRITE_PROTECTED | RW_FAILURE | NOT_DETECTED | NULL (belum pernah dicek / bukan dashcam blackvue)';
COMMENT ON COLUMN cameras.gps_no_signal_since IS 'Sejak kapan GPS terus-menerus tanpa sinyal (NULL = sinyal normal/belum ada masalah); dari proxy no_gps_count/no_gps_duration (command 0x0005), bukan koordinat asli.';
COMMENT ON COLUMN cameras.sim_status IS 'Ringkasan status SIM/modem dari command 0x000C (mis. INACTIVE, MODEM_OFF) | NULL = normal atau belum dicek.';
