CREATE TABLE "order_tags" (
    "id" UUID PRIMARY KEY,
    "tag_id" UUID REFERENCES "tags"("id")
);
