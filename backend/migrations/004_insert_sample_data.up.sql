-- Step 4: Insert sample products (permanent - won't be auto-deleted)
INSERT INTO products (prod_name, price, stock, ttl_expires_at) VALUES
    ('Wireless Mouse', 29.99, 100, NULL),
    ('Mechanical Keyboard', 89.99, 50, NULL),
    ('USB-C Hub', 49.99, 75, NULL),
    ('Monitor Stand', 39.99, 30, NULL),
    ('Webcam HD', 79.99, 25, NULL),
    ('Laptop Sleeve', 24.99, 200, NULL),
    ('Cable Organizer', 14.99, 150, NULL),
    ('Desk Lamp LED', 44.99, 40, NULL);

-- Insert sample orders (permanent - won't be auto-deleted)
INSERT INTO orders (product_id, quantity, total_price, status, shipping_address, notes, ttl_expires_at) VALUES
    (1, 2, 59.98, 'paid', '123 Main St, New York, NY 10001', 'Leave at door', NULL),
    (2, 1, 89.99, 'paid', '456 Oak Ave, San Francisco, CA 94102', '', NULL),
    (3, 3, 149.97, 'failed', '789 Pine Rd, Chicago, IL 60601', 'Gift wrap please', NULL);

-- Insert sample payments (permanent - won't be auto-deleted)
INSERT INTO payments (order_id, amount, status, card_number, card_last_four, ttl_expires_at) VALUES
    (1, 59.98, 'success', '4111111111111111', '1111', NULL),
    (2, 89.99, 'success', '5555555555554444', '4444', NULL),
    (3, 149.97, 'failed', '4000000000006969', '6969', NULL);
