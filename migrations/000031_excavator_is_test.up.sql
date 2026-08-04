-- Migration 000031: isolasi dashboard demo/testing dari data produksi. Excavator
-- ditandai is_test, user ditandai is_test_viewer -- user demo cuma lihat excavator
-- is_test=true, user biasa cuma lihat is_test=false, siapa pun yang login.
ALTER TABLE excavators ADD COLUMN is_test boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN is_test_viewer boolean NOT NULL DEFAULT false;

UPDATE excavators SET is_test = true WHERE code IN ('EXC-PC200-2', 'EXC-PC200-3');
