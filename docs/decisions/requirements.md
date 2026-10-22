# Order API Requirements

## Requirements

What the service delivers to the customer, what it must never violate, and what it needs to run in production.

The open items are left out on purpose: the project's learning goal is met.

### User requirements

| # | Requirement | Done |
| --- | --- | --- |
| R1 | The customer creates an order (`POST /orders`) with a shipping address and items, paid with their saved card | done |
| R2 | The order ships from a single warehouse: the closest to the address among those that have every item | done |
| R3 | Address not found: the customer gets an error and nothing is reserved or charged | done |
| R4 | No single warehouse can fill the order: the customer is told so and is not charged | done |
| R5 | Card declined: the customer is told so and the stock becomes available again | done |
| R6 | The same order sent again with the same key returns the first result | done |
| R7 | A key reused for a different order (or for the same order after a price change) is rejected | done |
| R8 | The customer can look up an order and see its status | done |

### Consistency guarantees

| # | Guarantee | Done |
| --- | --- | --- |
| G1 | Never charge without reserved stock | done |
| G2 | Never sell what isn't there: when two purchases of the last unit happen at once, only one gets it | done |
| G3 | Never charge the same order twice | done |
| G4 | Success requires reserved stock and an approved payment; any other outcome is a failure | done |
| G5 | A failure midway through processing never leaves the order half-saved | done |
| G6 | A payment timeout is not a failure: the payment is checked again before deciding | |
| G7 | No order is stuck forever: pending payments get resolved and stock held by abandoned orders is released | done |

### Operational requirements

| # | Requirement | Done |
| --- | --- | --- |
| O1 | Geocoding and payment are external APIs, simulated with mocks | done |
| O2 | Rate limiting: a single client can't flood the endpoint | |
| O3 | Maximum request body size, so the server's memory can't be exhausted | done |
| O4 | Server timeouts, so slow or idle connections don't stay open | done |
| O5 | A JSON body with extra data after the object is rejected | done |
| O6 | On shutdown, the service stops accepting requests and lets in-flight requests and running jobs finish before exiting | |

### Input validation

- [x] Idempotency key is required and well-formed
- [x] Customer and product ids are well-formed and exist
- [x] Shipping address is required
- [x] Items are not empty and every quantity is positive
- [x] Each product appears in at most one item

### Governance and observability requirements

| # | Requirement | Done |
| --- | --- | --- |
| V1 | Every log line from order processing carries the `order_id` | |
| V2 | Jobs that River discards after their retries run out raise an alert | |
| V3 | Operators can see logs and health checks for the service | |
| V4 | A client developer can integrate from the API documentation (endpoints, request and response shapes, error codes) | |

