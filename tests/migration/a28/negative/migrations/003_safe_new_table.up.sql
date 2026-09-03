CREATE TABLE categories (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL
);

ALTER TABLE categories ADD CONSTRAINT fk_categories_user FOREIGN KEY (user_id) REFERENCES users(id);
