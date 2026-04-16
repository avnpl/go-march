-- Create orders table with string IDs
CREATE TABLE IF NOT EXISTS orders (
    order_id STRING PRIMARY KEY,
    product_id STRING NOT NULL,
    quantity INT8 NOT NULL,
    total_price DECIMAL(10, 2) NOT NULL,
    order_time TIMESTAMPTZ NOT NULL DEFAULT now():::TIMESTAMPTZ,
    status STRING DEFAULT 'pending',
    shipping_address TEXT,
    notes TEXT,
    ttl_expires_at TIMESTAMPTZ,
    CONSTRAINT orders_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(prod_id)
);

-- Insert sample orders (permanent - won't be auto-deleted)
INSERT INTO orders (order_id, product_id, quantity, total_price, status, shipping_address, notes, ttl_expires_at) VALUES
    ('OR-AAA111', 'PR-A1B2C3', 2, 59.98, 'paid', '123 Main St, New York, NY 10001', 'Leave at door', NULL),
    ('OR-BBB222', 'PR-D4E5F6', 1, 89.99, 'paid', '456 Oak Ave, San Francisco, CA 94102', '', NULL),
    ('OR-CCC333', 'PR-G7H8I9', 3, 149.97, 'failed', '789 Pine Rd, Chicago, IL 60601', 'Gift wrap please', NULL);