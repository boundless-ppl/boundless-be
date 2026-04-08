ALTER TABLE subscriptions
ADD COLUMN normal_price_amount BIGINT NOT NULL DEFAULT 0 CHECK (normal_price_amount >= 0),
ADD COLUMN discount_price_amount BIGINT NOT NULL DEFAULT 0 CHECK (discount_price_amount >= 0);

-- Seed existing data with pricing
-- The Sprinter: 39000 discount, 149000 normal
-- The Scholar: 109000 discount, 399000 normal
-- The Visionary: 189000 discount, 749000 normal
UPDATE subscriptions SET 
  normal_price_amount = CASE package_key
    WHEN 'the_sprinter' THEN 149000
    WHEN 'the_scholar' THEN 399000
    WHEN 'the_visionary' THEN 749000
    ELSE 0
  END,
  discount_price_amount = CASE package_key
    WHEN 'the_sprinter' THEN 39000
    WHEN 'the_scholar' THEN 109000
    WHEN 'the_visionary' THEN 189000
    ELSE 0
  END
WHERE is_active = true;

-- Add comment
COMMENT ON COLUMN subscriptions.normal_price_amount IS 'Original price before discount (in IDR)';
COMMENT ON COLUMN subscriptions.discount_price_amount IS 'Price with discount applied (in IDR) - what user pays';
