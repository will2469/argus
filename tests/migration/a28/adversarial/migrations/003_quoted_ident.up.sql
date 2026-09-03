ALTER TABLE "orders" ADD CONSTRAINT "fk_orders_seller" FOREIGN KEY ("seller_id") REFERENCES "sellers"("id");
