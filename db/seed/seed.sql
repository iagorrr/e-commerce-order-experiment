-- Local seed data, loaded by compose after the schema on a fresh database.
-- Fixed ids so requests can be written ahead of time.

INSERT INTO customers (id, card_number) VALUES
    -- Card approved by the payment mock.
    ('01900000-0000-7000-8000-0000000000c1', '4242424242424242'),
    -- Card declined by the payment mock.
    ('01900000-0000-7000-8000-0000000000c2', '4000000000000002');

INSERT INTO products (id, price_cents) VALUES
    -- In every warehouse.
    ('01900000-0000-7000-8000-0000000000a1', 1500),
    -- Only in Berlin.
    ('01900000-0000-7000-8000-0000000000a2', 4000),
    -- One unit left, only in New York.
    ('01900000-0000-7000-8000-0000000000a3', 9900),
    -- In no warehouse.
    ('01900000-0000-7000-8000-0000000000a4', 2500);

-- The geocoding mock places an address at a point derived from its hash, so
-- each warehouse lists an address that the mock places closest to it.
INSERT INTO warehouses (id, lat, lng) VALUES
    -- New York; closest to "923 Broadway, New York".
    ('01900000-0000-7000-8000-0000000000e1', 40.71, -74.01),
    -- São Paulo; closest to "45 Avenida Paulista, São Paulo".
    ('01900000-0000-7000-8000-0000000000e2', -23.55, -46.63),
    -- Berlin; closest to "1020 Unter den Linden, Berlin".
    ('01900000-0000-7000-8000-0000000000e3', 52.52, 13.40);

INSERT INTO stock (warehouse_id, product_id, available) VALUES
    ('01900000-0000-7000-8000-0000000000e1', '01900000-0000-7000-8000-0000000000a1', 100),
    ('01900000-0000-7000-8000-0000000000e2', '01900000-0000-7000-8000-0000000000a1', 100),
    ('01900000-0000-7000-8000-0000000000e3', '01900000-0000-7000-8000-0000000000a1', 100),
    ('01900000-0000-7000-8000-0000000000e3', '01900000-0000-7000-8000-0000000000a2', 20),
    ('01900000-0000-7000-8000-0000000000e1', '01900000-0000-7000-8000-0000000000a3', 1);
