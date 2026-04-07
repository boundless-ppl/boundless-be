CREATE TABLE IF NOT EXISTS subscriptions (
  subscription_id UUID PRIMARY KEY,
  package_key TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  description TEXT NOT NULL,
  duration_months INTEGER NOT NULL CHECK (duration_months > 0),
  price_amount BIGINT NOT NULL CHECK (price_amount >= 0),
  benefits_json JSONB NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payments (
  payment_id UUID PRIMARY KEY,
  transaction_id TEXT NOT NULL UNIQUE,
  user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  subscription_id UUID NOT NULL REFERENCES subscriptions(subscription_id),

  package_name_snapshot TEXT NOT NULL,
  duration_months_snapshot INTEGER NOT NULL CHECK (duration_months_snapshot > 0),
  price_amount_snapshot BIGINT NOT NULL CHECK (price_amount_snapshot >= 0),
  benefits_snapshot_json JSONB NOT NULL,

  payment_channel TEXT NOT NULL DEFAULT 'qris_manual',
  qris_image_url TEXT NOT NULL,

  status TEXT NOT NULL CHECK (status IN ('pending', 'success', 'failed')),

  admin_note TEXT NULL,
  proof_document_id UUID NULL REFERENCES documents(document_id) ON DELETE SET NULL,

  verified_by TEXT NULL REFERENCES users(user_id) ON DELETE SET NULL,
  verified_at TIMESTAMPTZ NULL,
  paid_at TIMESTAMPTZ NULL,

  expired_at TIMESTAMPTZ NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_subscriptions (
  user_subscription_id UUID PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  subscription_id UUID NOT NULL REFERENCES subscriptions(subscription_id),

  source_payment_id UUID NOT NULL UNIQUE REFERENCES payments(payment_id) ON DELETE RESTRICT,

  package_name_snapshot TEXT NOT NULL,
  duration_months_snapshot INTEGER NOT NULL CHECK (duration_months_snapshot > 0),
  price_amount_snapshot BIGINT NOT NULL CHECK (price_amount_snapshot >= 0),

  start_date TIMESTAMPTZ NOT NULL,
  end_date TIMESTAMPTZ NOT NULL,

  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

  CHECK (end_date > start_date)
);
CREATE INDEX IF NOT EXISTS idx_payments_user_id ON payments(user_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_transaction_id ON payments(transaction_id);

CREATE INDEX IF NOT EXISTS idx_user_subscriptions_user_id ON user_subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_subscriptions_active ON user_subscriptions(user_id, end_date);

INSERT INTO subscriptions (
  subscription_id, package_key, name, description,
  duration_months, price_amount, benefits_json, is_active
)
VALUES
(
  '9a39f396-27c3-4f80-918d-06f31dadd4ef',
  'the_sprinter',
  'The Sprinter',
  'Cocok untuk yang tinggal poles dokumen terakhir.',
  1,
  149000,
  '["GlobalMatch AI","DreamTracker","Auto-fill","Priority support","Advanced analytics"]',
  TRUE
),
(
  '38f4e258-6715-45c5-bf17-7ce733178ce5',
  'the_scholar',
  'The Scholar',
  'Harga terbaik untuk hasil maksimal.',
  3,
  299000,
  '["GlobalMatch AI","DreamTracker","Auto-fill","Priority support","Advanced analytics","Unlimited search","Personalized recommendations"]',
  TRUE
),
(
  '2f5fdbf6-11d4-4b2f-90d4-c8ffbf7d43b0',
  'the_visionary',
  'The Visionary',
  'Paling hemat jangka panjang.',
  6,
  499000,
  '["GlobalMatch AI","DreamTracker","Auto-fill","Priority support","Advanced analytics","Unlimited search","Personalized recommendations","Full support"]',
  TRUE
)
ON CONFLICT (subscription_id) DO NOTHING;
