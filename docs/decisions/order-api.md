# Order API Decisions

Key technical decisions for the order API in the
[scenario](../scenario.md).

## Stack

- **Language and libraries:** Go with the `net/http` standard-library router,
  `pgx` with hand-written queries (no sqlc or ORM), and River for background
  jobs on the same Postgres.
- **Database:** PostgreSQL 18 (Alpine image), with IDs from `uuidv7()`. The
  schema comes from an init script, `db/migrations/001_schema.sql`, that runs
  only on an empty database, followed by the local seed data in
  `db/seed/seed.sql`; River creates its own tables at startup. The
  connection string is hardcoded.
- **Running:** `just up` (Docker Compose) starts Postgres and one API
  instance.

## API

- **`POST /orders`:** validates the request, prices the items from the
  database in integer cents, and stores the order as `PENDING` together with
  its first job in one transaction. It answers `201` with the order's id and
  status without waiting for processing. An invalid request, including one
  that lists the same product twice, gets `400`; an unknown customer or
  product gets `422`. The `Idempotency-Key` header must be a version 7 UUID.
- **`GET /orders/{id}`:** returns the order with its status, items and total,
  plus its location and warehouse once they are known.

## Processing

- **Jobs:** processing runs as four River jobs, one per step: geocode,
  allocate, charge and release. Each step that calls an external API retries
  on its own, so a payment retry never geocodes again. Every job carries only
  the order id and reads the order, with its lines, from the database.
- **Transitions:** each job moves the order from one status to the next,
  saves what the step produced, enqueues the next job and completes itself in
  one transaction. A job that finds the order already past its step was a
  duplicate and only completes.
- **Geocode:** the geocoder is called outside the transaction. The point is
  saved on the order as it moves `PENDING → GEOCODED`. An address that isn't
  found moves it to `FAILED`.
- **Warehouse selection:** a single SQL query picks the warehouse closest by
  haversine distance among those that have every item available. Orders are
  never split. None found moves the order to `FAILED`.
- **Reservation:** optimistic. Each item is taken with a conditional
  `UPDATE` that only succeeds while enough stock is available, in product
  order so concurrent orders can't deadlock. The warehouse is saved on the
  order as it moves `GEOCODED → RESERVED`. An order that loses the race to
  another rolls back and picks a warehouse again, up to three times, before
  leaving it to River's retry.
- **Charge:** the order id is the payment's idempotency key, so retries
  charge once. A declined card moves the order to `RELEASING`; any other
  error is retried.
- **Release:** the reserved quantities go back to the order's warehouse as
  it moves `RELEASING → FAILED`.
- **Retries:** every job gets River's default of 25 attempts, with waits
  that grow until they span about three weeks, so outages of the geocoder or
  the gateway heal on their own.
- **Stuck orders:** every 5 minutes a periodic job looks for order jobs River
  discarded after their last attempt, whose order is still waiting on them.
  An order still waiting to be geocoded or allocated holds no stock and moves
  to `FAILED`. A discarded charge or release is retried, forever: repeating
  the charge is safe because of its idempotency key, and reserved stock must
  always be given back. Each case is logged with the order id.
- **Mocks:** geocoding and payment run on deterministic mocks wired in
  `main`. An address containing `not found` is not found; the card
  `4000000000000002` is declined.
