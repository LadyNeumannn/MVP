CREATE TABLE restaurants (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    webhook_url TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE menu_items (
    id            UUID PRIMARY KEY,
    restaurant_id UUID NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    price_cents   BIGINT NOT NULL CHECK (price_cents >= 0),
    available     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_menu_items_restaurant ON menu_items(restaurant_id);

CREATE TYPE order_status AS ENUM (
    'created', 'confirmed', 'preparing', 'ready', 'delivered', 'cancelled'
);

CREATE TABLE orders (
    id            UUID PRIMARY KEY,
    restaurant_id UUID NOT NULL REFERENCES restaurants(id),
    customer_name TEXT NOT NULL,
    status        order_status NOT NULL DEFAULT 'created',
    total_cents   BIGINT NOT NULL CHECK (total_cents >= 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_restaurant_status ON orders(restaurant_id, status);

CREATE TABLE order_items (
    id                    UUID PRIMARY KEY,
    order_id              UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id          UUID NOT NULL REFERENCES menu_items(id),
    name_snapshot         TEXT NOT NULL,
    price_cents_snapshot  BIGINT NOT NULL,
    quantity              INT NOT NULL CHECK (quantity > 0)
);
CREATE INDEX idx_order_items_order ON order_items(order_id);
