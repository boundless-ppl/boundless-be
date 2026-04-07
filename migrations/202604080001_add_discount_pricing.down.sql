ALTER TABLE subscriptions
DROP COLUMN IF EXISTS discount_price_amount,
DROP COLUMN IF EXISTS normal_price_amount;
