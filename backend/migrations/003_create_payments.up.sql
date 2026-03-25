-- Step 3: Create payments table
CREATE TABLE IF NOT EXISTS payments (
    payment_id INT8 NOT NULL DEFAULT unique_rowid(),
    order_id INT8 NOT NULL,
    amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    card_number VARCHAR(255),
    card_last_four VARCHAR(4),
    created_at TIMESTAMP DEFAULT now():::TIMESTAMP,
    ttl_expires_at TIMESTAMP,
    CONSTRAINT payments_order_id_fkey FOREIGN KEY (order_id) REFERENCES orders(order_id)
);
