// Command api serves the order API over HTTP and runs the order jobs in the
// background.
package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	customerpg "orders-api/internal/customer/postgres"
	geocodingmock "orders-api/internal/geocoding/mock"
	"orders-api/internal/order"
	orderhttp "orders-api/internal/order/http"
	orderpg "orders-api/internal/order/postgres"
	orderriver "orders-api/internal/order/river"
	paymentmock "orders-api/internal/payment/mock"
	productpg "orders-api/internal/product/postgres"
	warehousepg "orders-api/internal/warehouse/postgres"
)

const databaseURL = "postgres://orders:orders@postgres:5432/orders"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return err
	}

	// The service enqueues jobs, and a worker runs the service, so the service
	// enqueues with an insert-only client instead of the one that works jobs.
	inserter, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		return err
	}
	warehouses := warehousepg.NewStore(pool)
	uow := orderpg.NewUnitOfWork(pool, inserter, warehouses)

	svc := order.NewService(
		orderpg.NewStore(pool),
		productpg.NewStore(pool),
		customerpg.NewStore(pool),
		warehouses,
		geocodingmock.Geocoder{},
		paymentmock.Gateway{},
		uow,
	)

	workers := river.NewWorkers()
	orderriver.AddWorkers(workers, svc)
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		PeriodicJobs: orderriver.PeriodicJobs(),
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 25}},
		Workers:      workers,
	})
	if err != nil {
		return err
	}
	if err := riverClient.Start(ctx); err != nil {
		return err
	}

	handler := orderhttp.NewHandler(svc)

	// Timeouts so slow or idle clients can't hold connections open.
	server := &http.Server{
		Addr:              ":8080",
		Handler:           NewRouter(handler),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Printf("listening on %s", server.Addr)
	return server.ListenAndServe()
}
