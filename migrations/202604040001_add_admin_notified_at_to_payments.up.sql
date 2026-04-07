ALTER TABLE payments
ADD COLUMN IF NOT EXISTS admin_notified_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_payments_admin_notified_at
ON payments(admin_notified_at);
