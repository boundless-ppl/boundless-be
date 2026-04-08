-- Add price snapshot columns to payments table for complete pricing audit
ALTER TABLE payments 
ADD COLUMN normal_price_snapshot BIGINT NULL,
ADD COLUMN discount_price_snapshot BIGINT NULL CHECK (discount_price_snapshot >= 0);

-- Update existing payments to store snapshot prices
UPDATE payments SET
  discount_price_snapshot = price_amount_snapshot,
  normal_price_snapshot = CASE 
    WHEN price_amount_snapshot > 0 THEN (price_amount_snapshot * 100) / 10
    ELSE price_amount_snapshot
  END
WHERE normal_price_snapshot IS NULL AND price_amount_snapshot > 0;

COMMENT ON COLUMN payments.normal_price_snapshot IS 'Original price before discount (in IDR) - captured at time of payment';
COMMENT ON COLUMN payments.discount_price_snapshot IS 'Price with discount applied (in IDR) - what user paid, captured at time of payment';
