CREATE TABLE public.audit_logs (
    id UUID PRIMARY KEY,
    action_time TIMESTAMP NOT NULL
);
