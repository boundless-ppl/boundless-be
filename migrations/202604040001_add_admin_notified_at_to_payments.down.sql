DROP INDEX IF EXISTS idx_payments_admin_notified_at;

ALTER TABLE payments
DROP COLUMN IF EXISTS admin_notified_at;
