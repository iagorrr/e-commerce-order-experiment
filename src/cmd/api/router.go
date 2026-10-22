package main

import (
	"net/http"

	orderhttp "orders-api/internal/order/http"
)

// NewRouter returns the API's routes.
func NewRouter(orders *orderhttp.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", orders.Create)
	mux.HandleFunc("GET /orders/{id}", orders.Get)
	return mux
}
