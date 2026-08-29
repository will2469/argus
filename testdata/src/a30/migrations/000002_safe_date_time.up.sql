CREATE TABLE IF NOT EXISTS work_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shift_date DATE NOT NULL,
    start_time TIME NOT NULL
);
