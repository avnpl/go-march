-- Create payments table with string IDs
CREATE TABLE IF NOT EXISTS payments (
    payment_id STRING PRIMARY KEY,
    order_id STRING NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    status STRING DEFAULT 'pending',
    card_number STRING,
    card_last_four STRING,
    created_at TIMESTAMPTZ DEFAULT now():::TIMESTAMPTZ,
    ttl_expires_at TIMESTAMPTZ,
    CONSTRAINT payments_order_id_fkey FOREIGN KEY (order_id) REFERENCES orders(order_id)
);

-- Insert sample payments (permanent - won't be auto-deleted)
INSERT INTO payments (payment_id, order_id, amount, status, card_number, card_last_four, ttl_expires_at) VALUES
    ('PA-XXX111', 'OR-AAA111', 59.98, 'success', '4111111111111111', '1111', NULL),
    ('PA-YYY222', 'OR-BBB222', 89.99, 'success', '5555555555554444', '4444', NULL),
    ('PA-ZZZ333', 'OR-CCC333', 149.97, 'failed', '4000000000006969', '6969', NULL);