ALTER TABLE public.logs ADD CONSTRAINT chk_level CHECK (level IN ('INFO', 'WARN', 'ERROR'));
