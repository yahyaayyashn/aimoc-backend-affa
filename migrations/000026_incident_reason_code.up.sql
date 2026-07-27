-- Migration: 000026_incident_reason_code
-- Klasifikasi jenis gangguan pada camera_incidents -- sebelumnya cuma generic
-- connection-loss (last_seen_at basi). Sekarang device-health check (SD/RTC/GPS/SIM,
-- lihat migration 000025 + device_health.go) juga bisa memicu incident, dengan
-- reason_code yang menjelaskan jenisnya. Nullable: incident lama tetap dianggap
-- CONNECTION_LOST (perilaku existing, tidak berubah).

ALTER TABLE camera_incidents
    ADD COLUMN IF NOT EXISTS reason_code varchar(40) NULL;

COMMENT ON COLUMN camera_incidents.reason_code IS
    'CONNECTION_LOST (default/lama) | SD_FULL | SD_WRITE_FAILURE | SD_WRITE_PROTECTED | SD_RW_FAILURE | SD_NOT_DETECTED | RTC_LOW_BATTERY | GPS_NO_SIGNAL | SIM_INACTIVE | FLEET_DISCONNECTED';
