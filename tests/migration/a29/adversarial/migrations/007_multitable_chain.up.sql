CREATE TABLE line_items (
    id UUID PRIMARY KEY,
    warehouse_id UUID REFERENCES warehouses(id)
);
