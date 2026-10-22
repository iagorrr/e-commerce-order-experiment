# Scenario

This project is an experiment built around a hypothetical scenario: a backend
service for an e-commerce platform, with a minimal **order management API**.
This document describes that scenario and is the guide for development.

## Requirements

### `POST /orders`

Creates an order. Called by the UI as customers place orders.

An order has:

- a **customer**
- a **shipping address**
- a list of **items** (products and quantities)

### Warehouse selection

- An order must be filled from a **single warehouse**, so the service needs to
  find a warehouse that has *all* the requested products.
- If multiple warehouses fit, pick the one **closest to the shipping address**.

### Geocoding

Converting an address to latitude/longitude would usually go through a
third-party geocoding API. **It is mocked.**

### Payment

On creating the order, the service should call an external **payment API**,
which is mocked. The payments API takes as input:

| Field              | Description                  |
| ------------------ | ---------------------------- |
| Credit card number | The customer's card number   |
| Amount             | The amount to charge         |
| Description        | A description of the charge  |

> In the real world we wouldn't want to handle people's credit card numbers,
> and the payment integration would be more complicated than a simple API
> request. Security concerns are left out of scope for simplicity, so in this
> scenario it's that simple.

## Ground rules

- **Scope:** no full app/system. For example, no APIs for managing
  customers/warehouses/products, and no auth. Only the functionality specified
  above.
- **Production-ready:** that functionality should be production-ready. It uses
  a **real database** and treats data storage and management with the same
  rigor as a high-traffic production system. The goal is to simulate
  real-world engineering work as closely as possible.
- **Ambiguity:** the requirements are intentionally short. Gaps are filled with
  reasonable design decisions.
