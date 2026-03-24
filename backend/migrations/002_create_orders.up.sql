-- Create orders table
CREATE TABLE IF NOT EXISTS orders (
    order_id BIGSERIAL PRIMARY KEY,
    product_id BIGINT NOT NULL REFERENCES products(prod_id),
    quantity INT NOT NULL,
    total_price DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    shipping_address TEXT,
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    ttl_expires_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() + INTERVAL '3 hours')
);

-- Insert sample orders
INSERT INTO orders (product_id, quantity, total_price, status, shipping_address, notes) VALUES
    (1, 2, 59.98, 'paid', '123 Main St, New York, NY 10001', 'Leave at door'),
    (2, 1, 89.99, 'paid', '456 Oak Ave, San Francisco, CA 94102', ''),
    (3, 3, 149.97, 'failed', '789 Pine Rd, Chicago, IL 60601', 'Gift wrap please');