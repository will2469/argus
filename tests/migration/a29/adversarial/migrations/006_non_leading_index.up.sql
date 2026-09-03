CREATE TABLE likes (
    id UUID PRIMARY KEY,
    post_id UUID REFERENCES posts(id),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_likes_date_post ON likes (created_at, post_id);
