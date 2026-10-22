# Running it

Needs Docker with Compose and [just](https://github.com/casey/just).

```sh
just up      # start Postgres and the API on localhost:8080
just down    # stop everything, keeping the database
just reset   # wipe the database and start again
```

A fresh database starts with the seed data in
[db/seed/seed.sql](../db/seed/seed.sql). Every id is
`01900000-0000-7000-8000-0000000000` followed by the code below:

| Code | What it is |
| --- | --- |
| `c1` | Customer whose card is approved |
| `c2` | Customer whose card is declined |
| `a1` | Product in every warehouse |
| `a2` | Product only in Berlin |
| `a3` | Product with one unit left, in New York |
| `a4` | Product in no warehouse |
| `e1`, `e2`, `e3` | Warehouses in New York, São Paulo and Berlin |

The geocoder is a mock that places each address at a point derived from its
text, so these addresses ship from the matching warehouse:
`923 Broadway, New York`, `45 Avenida Paulista, São Paulo` and
`1020 Unter den Linden, Berlin`. An address containing `not found` is not
found.

Place an order, then look it up with the returned id:

```sh
curl -i localhost:8080/orders \
  -H 'Idempotency-Key: 01900000-0000-7000-8000-000000000001' \
  -d '{"customer_id": "01900000-0000-7000-8000-0000000000c1",
       "shipping_address": "1020 Unter den Linden, Berlin",
       "items": [{"product_id": "01900000-0000-7000-8000-0000000000a2", "quantity": 1}]}'

curl localhost:8080/orders/<id>
```

Sending the same request again with the same key returns the same order.
