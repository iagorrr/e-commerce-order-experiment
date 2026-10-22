// Package http serves the order API over HTTP.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"

	"orders-api/internal/order"
	"orders-api/internal/product"
)

type orderService interface {
	Create(ctx context.Context, n order.NewOrder) (order.Order, bool, error)
	Get(ctx context.Context, id uuid.UUID) (order.Order, error)
}

// Handler serves the order HTTP endpoints.
type Handler struct {
	orders orderService
}

// NewHandler returns a Handler backed by orders.
func NewHandler(orders orderService) *Handler {
	return &Handler{orders: orders}
}

// createOrderRequest is the body of POST /orders.
type createOrderRequest struct {
	CustomerID      string            `json:"customer_id"`
	ShippingAddress string            `json:"shipping_address"`
	Items           []createOrderItem `json:"items"`
}

type createOrderItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

// maxBodyBytes caps the POST /orders body, so a huge request can't exhaust
// the server's memory.
const maxBodyBytes = 1 << 20

// Create handles POST /orders: it stores the order and returns its id and
// status without waiting for processing.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	n, err := parseRequest(r)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body must be at most %d bytes", maxBodyBytes))
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	o, replayed, err := h.orders.Create(r.Context(), n)
	if isInvalidOrder(err) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, order.ErrKeyReused) {
		writeError(w, http.StatusUnprocessableEntity, "Idempotency-Key was already used for a different request")
		return
	}
	if errors.Is(err, order.ErrCustomerNotFound) || errors.Is(err, product.ErrNotFound) {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err != nil {
		log.Printf("creating order: %v", err)
		writeError(w, http.StatusInternalServerError, "could not create order: "+err.Error())
		return
	}

	if replayed {
		w.Header().Set("Idempotent-Replayed", "true")
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": o.ID, "status": o.Status})
}

// isInvalidOrder reports whether err means the order failed the core's
// validation.
func isInvalidOrder(err error) bool {
	for _, target := range []error{order.ErrInvalidID, order.ErrInvalidItems, order.ErrInvalidShippingAddress} {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}

// orderResponse is the body of GET /orders/{id}. WarehouseID and Location are
// omitted until the order has them.
type orderResponse struct {
	ID              uuid.UUID         `json:"id"`
	Status          order.Status      `json:"status"`
	CustomerID      uuid.UUID         `json:"customer_id"`
	ShippingAddress string            `json:"shipping_address"`
	TotalCents      int64             `json:"total_cents"`
	Items           []lineResponse    `json:"items"`
	WarehouseID     *uuid.UUID        `json:"warehouse_id,omitempty"`
	Location        *locationResponse `json:"location,omitempty"`
}

type lineResponse struct {
	ProductID      uuid.UUID `json:"product_id"`
	Quantity       int       `json:"quantity"`
	UnitPriceCents int64     `json:"unit_price_cents"`
}

type locationResponse struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// newOrderResponse converts o into the GET /orders/{id} body.
func newOrderResponse(o order.Order) orderResponse {
	resp := orderResponse{
		ID:              o.ID,
		Status:          o.Status,
		CustomerID:      o.CustomerID,
		ShippingAddress: o.ShippingAddress,
		TotalCents:      o.TotalCents,
		Items:           make([]lineResponse, len(o.Items)),
	}
	for i, l := range o.Items {
		resp.Items[i] = lineResponse{ProductID: l.ProductID, Quantity: l.Quantity, UnitPriceCents: l.UnitPriceCents}
	}
	if o.WarehouseID != uuid.Nil {
		resp.WarehouseID = &o.WarehouseID
	}
	if o.Location != nil {
		resp.Location = &locationResponse{Lat: o.Location.Lat, Lng: o.Location.Lng}
	}
	return resp
}

// Get handles GET /orders/{id}: it returns the order as it is now.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "order id must be a UUID")
		return
	}

	o, err := h.orders.Get(r.Context(), id)
	if errors.Is(err, order.ErrInvalidID) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, order.ErrNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		log.Printf("getting order %s: %v", id, err)
		writeError(w, http.StatusInternalServerError, "could not get order: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, newOrderResponse(o))
}

func parseRequest(r *http.Request) (order.NewOrder, error) {
	idempotencyKey, ok := parseUUID(r.Header.Get("Idempotency-Key"))
	if !ok {
		return order.NewOrder{}, errors.New("Idempotency-Key must be a UUID")
	}

	var req createOrderRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		return order.NewOrder{}, fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return order.NewOrder{}, err
		}
		return order.NewOrder{}, errors.New("invalid JSON body: unexpected data after the object")
	}
	n, err := req.toNewOrder()
	if err != nil {
		return order.NewOrder{}, err
	}
	n.IdempotencyKey = idempotencyKey
	return n, nil
}

// toNewOrder converts req into the service's input. It only parses; the
// service validates the result.
func (req createOrderRequest) toNewOrder() (order.NewOrder, error) {
	customerID, ok := parseUUID(req.CustomerID)
	if !ok {
		return order.NewOrder{}, errors.New("customer_id must be a UUID")
	}

	items := make([]order.Item, len(req.Items))
	for i, it := range req.Items {
		productID, ok := parseUUID(it.ProductID)
		if !ok {
			return order.NewOrder{}, errors.New("each item's product_id must be a UUID")
		}
		items[i] = order.Item{ProductID: productID, Quantity: it.Quantity}
	}
	return order.NewOrder{CustomerID: customerID, ShippingAddress: req.ShippingAddress, Items: items}, nil
}

// parseUUID parses s as a UUID of any version in its canonical
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx form. An empty s is uuid.Nil, so the
// core reports the value as missing.
func parseUUID(s string) (uuid.UUID, bool) {
	if s == "" {
		return uuid.Nil, true
	}
	id, err := uuid.Parse(s)
	if err != nil || len(s) != 36 {
		return uuid.UUID{}, false
	}
	return id, true
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writing response: %v", err)
	}
}
