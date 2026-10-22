-- Each customer has one saved card. Production would store a provider token
-- instead of the card number.
CREATE TABLE customers (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    card_number text NOT NULL
);

CREATE TABLE products (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    price_cents bigint NOT NULL CHECK (price_cents > 0)
);

CREATE TABLE warehouses (
    id  uuid PRIMARY KEY DEFAULT uuidv7(),
    lat double precision NOT NULL CHECK (lat BETWEEN -90 AND 90),
    lng double precision NOT NULL CHECK (lng BETWEEN -180 AND 180)
);

CREATE TABLE stock (
    warehouse_id uuid NOT NULL REFERENCES warehouses (id),
    product_id   uuid NOT NULL REFERENCES products (id),
    available    integer NOT NULL CHECK (available >= 0),
    PRIMARY KEY (warehouse_id, product_id)
);

CREATE TABLE orders (
    id                uuid PRIMARY KEY DEFAULT uuidv7(),
    customer_id       uuid NOT NULL REFERENCES customers (id),
    idempotency_key   uuid NOT NULL,
    shipping_address  text NOT NULL,
    lat               double precision,
    lng               double precision,
    warehouse_id      uuid REFERENCES warehouses (id),
    status            text NOT NULL CHECK (status IN ('PENDING', 'GEOCODED', 'RESERVED', 'RELEASING', 'CONFIRMED', 'FAILED')),
    total_cents       bigint NOT NULL,
    UNIQUE (customer_id, idempotency_key),
    CHECK ((lat IS NULL) = (lng IS NULL))
    -- todo: also require lat and lng from GEOCODED on. Needs the status and
    -- the location written in one UPDATE, since a CHECK runs per statement.
);

CREATE TABLE order_items (
    order_id         uuid NOT NULL REFERENCES orders (id),
    product_id       uuid NOT NULL REFERENCES products (id),
    quantity         integer NOT NULL CHECK (quantity > 0),
    unit_price_cents bigint NOT NULL,
    PRIMARY KEY (order_id, product_id)
);
