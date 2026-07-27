-- Migration: 000019_camera_heartbeat (DOWN)
ALTER TABLE cameras
    DROP COLUMN IF EXISTS last_activity_status,
    DROP COLUMN IF EXISTS last_activity_source;
