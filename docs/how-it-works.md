# How it works

- **Instant response:** placing an order never waits on geocoding, stock or
  payment.
- **Never oversells:** each unit in stock is sold at most once; no two orders
  ever reserve the same unit, even when they arrive at the same time.
- **Never charges by mistake:** the card is charged only after the stock is
  reserved, and never twice for the same order.
- **Gives stock back:** when a card is declined, the reserved items are
  available again right away.
- **No half-done orders:** a crash or a failing external service never leaves
  an order partly saved; the failed step is simply tried again.
- **Safe to resend:** a client that retries a request, after a timeout for
  example, gets its original order back instead of a duplicate.

The design decisions behind each of these are in
[decisions/order-api.md](decisions/order-api.md), and the code
layout in [decisions/code-architecture.md](decisions/code-architecture.md).
