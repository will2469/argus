CREATE TABLE invoices (
    id UUID PRIMARY KEY,
    customer_id UUID REFERENCES customers(id)
);
