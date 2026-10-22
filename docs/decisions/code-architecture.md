# Code Architecture and Principles

How the code for the [order API](order-api.md) is organized, and how
SOLID is applied to it.

## External references

- **Code conventions:** the [Google Go Style Guide](https://google.github.io/styleguide/go/)
  (Guide, Decisions and Best Practices).
- **Go conventions:** naming and documentation rules are in
  [go-conventions.md](go-conventions.md).
- **Architecture:** Go-idiomatic hexagonal (ports and adapters), organized
  **package by feature**. Each feature (`order`, `warehouse`, `payment`,
  `geocoding`, …) has:
  - **The core in its root package** (`order`, …): its entities, business
    rules, use cases, and the ports it needs from the outside world.
  - **One subpackage per adapter**, named after its technology: `http` for
    the HTTP handler, `river` for the job workers, `postgres` for the store
    (hand-written `pgx` queries), `mock` for a stand-in of an external
    service.

  Ports are consumer-defined interfaces: each one lives in the package that
  calls it and lists only the methods that package uses. Driven ports, like
  `order.Store`, live in the core. Driving ports live in their adapter: the
  HTTP handler declares an unexported interface for the order use cases,
  which `*order.Service` satisfies implicitly.

  The root package never imports its adapters or their technologies (`pgx`,
  `river`, `net/http`); adapters import the root package. `cmd/api/main.go`
  builds the adapters and wires them into the core. There are no layer
  packages (no shared `httpapi` or `postgres`) and no DI container.
- **Data access:** every package that uses Postgres has a `postgres`
  subpackage whose `Store` struct holds a `db.DBTX` (the querier interface
  in `internal/db`, satisfied by `*pgxpool.Pool` and `pgx.Tx`), built on the
  pool with `NewStore(pool)`. It implements the feature's own `Store`
  interface, so services and workers never see pgx or SQL. A store used
  inside a transaction also has `WithTx(tx)`, which returns a copy bound to
  that transaction (sqlc's pattern); today `order.Store` and
  `warehouse.Store`.
