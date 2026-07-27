-- Migration: 000018_loading_cycle_source (DOWN)
ALTER TABLE loading_cycles DROP COLUMN IF EXISTS source;
