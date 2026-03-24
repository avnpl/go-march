-- Create payments table
CREATE TABLE IF NOT EXISTS payments (
    payment_id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(order_id),
    amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    card_number VARCHAR(255),
    card_last_four VARCHAR(4),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    ttl_expires_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() + INTERVAL '3 hours')
);

-- Insert sample payments
INSERT INTO payments (order_id, amount, status, card_number, card_last_four) VALUES
    (1, 59.98, 'success', '4111111111111111', '1111'),
    (2, 89.99, 'success', '5555555555554444', '4444'),
    (3, 149.97, 'failed', '4000000000006969', '6969');