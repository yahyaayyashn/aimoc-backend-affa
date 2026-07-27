ALTER TABLE cameras
    DROP COLUMN IF EXISTS sd_status,
    DROP COLUMN IF EXISTS sd_avail_kb,
    DROP COLUMN IF EXISTS gps_no_signal_since,
    DROP COLUMN IF EXISTS rtc_low_battery,
    DROP COLUMN IF EXISTS sim_status,
    DROP COLUMN IF EXISTS health_checked_at;
