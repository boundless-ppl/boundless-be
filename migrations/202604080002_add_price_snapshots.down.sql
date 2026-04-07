ALTER TABLE payments
DROP COLUMN IF EXISTS discount_price_snapshot,
DROP COLUMN IF EXISTS normal_price_snapshot;
