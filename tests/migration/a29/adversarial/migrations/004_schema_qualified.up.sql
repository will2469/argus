CREATE TABLE audit_records (
    id UUID PRIMARY KEY,
    user_id UUID REFERENCES public.users(id)
);
