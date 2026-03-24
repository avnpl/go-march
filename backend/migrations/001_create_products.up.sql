-- Create products table
CREATE TABLE IF NOT EXISTS products (
    prod_id BIGSERIAL PRIMARY KEY,
    prod_name VARCHAR(255) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    ttl_expires_at TIMESTAMP WITH TIME ZONE DEFAULT (NOW() + INTERVAL '3 hours')
);

-- Insert sample products
INSERT INTO products (prod_name, price, stock) VALUES
    ('Wireless Mouse', 29.99, 100),
    ('Mechanical Keyboard', 89.99, 50),
    ('USB-C Hub', 49.99, 75),
    ('Monitor Stand', 39.99, 30),
    ('Webcam HD', 79.99, 25),
    ('Laptop Sleeve', 24.99, 200),
    ('Cable Organizer', 14.99, 150),
    ('Desk Lamp LED', 44.99, 40);