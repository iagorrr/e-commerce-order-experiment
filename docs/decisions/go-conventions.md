# Go Conventions

Naming, documentation and error rules the Go code follows, from the
[Google Go Style Guide](https://google.github.io/styleguide/go/) and
*The Go Programming Language* (Donovan and Kernighan).

## Initialisms

An initialism or acronym in a name keeps one case for all its letters:
`orderID`, `OrderID`, `parseURL`, `HTTPClient`, never `orderId` or
`parseUrl`.

## Package comments

Every package has a doc comment right above its `package` clause, starting
with `Package <name>` and summarizing what the package does:

```go
// Package geocoding converts free-text addresses into coordinates.
package geocoding
```

## Error wrapping

The innermost error wraps the sentinel, and each layer above adds context
with a single `%w`, so callers can still match the sentinel with
`errors.Is`:

```go
// Item.Validate
return fmt.Errorf("%w: product id must be a version 7 UUID", ErrInvalidID)

// NewOrder.Validate
return fmt.Errorf("items[%d]: %w", i, err)
```
