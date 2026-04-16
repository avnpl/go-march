-- Create products table with string IDs
CREATE TABLE IF NOT EXISTS products (
    prod_id STRING PRIMARY KEY,
    prod_name VARCHAR(100) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INT8 NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now():::TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now():::TIMESTAMPTZ,
    ttl_expires_at TIMESTAMPTZ
);

-- Insert sample products (permanent - won't be auto-deleted)
INSERT INTO products (prod_id, prod_name, price, stock, ttl_expires_at) VALUES
    ('PR-A1B2C3', 'Wireless Mouse', 29.99, 100, NULL),
    ('PR-D4E5F6', 'Mechanical Keyboard', 89.99, 50, NULL),
    ('PR-G7H8I9', 'USB-C Hub', 49.99, 75, NULL),
    ('PR-J0K1L2', 'Monitor Stand', 39.99, 30, NULL),
    ('PR-M3N4O5', 'Webcam HD', 79.99, 25, NULL),
    ('PR-P6Q7R8', 'Laptop Sleeve', 24.99, 200, NULL),
    ('PR-S9T0U1', 'Cable Organizer', 14.99, 150, NULL),
    ('PR-V2W3X4', 'Desk Lamp LED', 44.99, 40, NULL);