-- Enable TTL using custom column (ttl_expires_at)
-- Rows will be deleted when current time > ttl_expires_at
-- Sample data has ttl_expires_at = NULL (never expires)

ALTER TABLE products SET (ttl_expiration_expression = 'ttl_expires_at');
ALTER TABLE orders SET (ttl_expiration_expression = 'ttl_expires_at');
ALTER TABLE payments SET (ttl_expiration_expression = 'ttl_expires_at');