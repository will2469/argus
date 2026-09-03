create table shipments (
    id uuid primary key,
    order_id uuid references orders(id)
);
