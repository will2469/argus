ALTER TABLE comments ADD CONSTRAINT fk_post FOREIGN KEY (post_id) REFERENCES posts(id);
