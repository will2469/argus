ALTER TABLE orders ADD COLUMN buyer_id INT;
ALTER TABLE orders ADD CONSTRAINT fk_orders_buyer FOREIGN KEY (buyer_id) REFERENCES buyers(id);
