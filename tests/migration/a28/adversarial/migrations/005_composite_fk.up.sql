ALTER TABLE order_items ADD CONSTRAINT fk_order_item FOREIGN KEY (order_id, item_id) REFERENCES items(order_id, id);
