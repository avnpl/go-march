-- Step 2: Add missing columns to orders table
ALTER TABLE orders ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'pending';
ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipping_address TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS notes TEXT;
ALTER TABLE orders ADD COLUMN IF NOT EXISTS ttl_expires_at TIMESTAMP;

-- Add TTL column to products table
ALTER TABLE products ADD COLUMN IF NOT EXISTS ttl_expires_at TIMESTAMP;
