// Package river runs the order steps as River jobs. Each worker only reads its
// job and calls the order service, which holds the logic.
package river

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"orders-api/internal/order"
)

type orderService interface {
	GeocodeAddress(ctx context.Context, id uuid.UUID) error
	AllocateStock(ctx context.Context, id uuid.UUID) error
	ChargePayment(ctx context.Context, id uuid.UUID) error
	ReleaseStock(ctx context.Context, id uuid.UUID) error
	Discarded(ctx context.Context, id uuid.UUID, waiting order.Status) (bool, error)
}

// Job arguments. Each job carries only the order it moves.

type GeocodeAddressArgs struct {
	OrderID uuid.UUID `json:"order_id"`
}

func (GeocodeAddressArgs) Kind() string { return "geocode_address" }

type AllocateStockArgs struct {
	OrderID uuid.UUID `json:"order_id"`
}

func (AllocateStockArgs) Kind() string { return "allocate_stock" }

type ChargePaymentArgs struct {
	OrderID uuid.UUID `json:"order_id"`
}

func (ChargePaymentArgs) Kind() string { return "charge_payment" }

type ReleaseStockArgs struct {
	OrderID uuid.UUID `json:"order_id"`
}

func (ReleaseStockArgs) Kind() string { return "release_stock" }

type SweepStuckOrdersArgs struct{}

func (SweepStuckOrdersArgs) Kind() string { return "sweep_stuck_orders" }

// waitingStatus is the status an order waits in while each job kind runs.
var waitingStatus = map[string]order.Status{
	GeocodeAddressArgs{}.Kind(): order.StatusPending,
	AllocateStockArgs{}.Kind():  order.StatusGeocoded,
	ChargePaymentArgs{}.Kind():  order.StatusReserved,
	ReleaseStockArgs{}.Kind():   order.StatusReleasing,
}

// NewJob returns the job that moves an order out of s.
func NewJob(s order.Status, orderID uuid.UUID) (river.JobArgs, error) {
	switch s {
	case order.StatusPending:
		return GeocodeAddressArgs{OrderID: orderID}, nil
	case order.StatusGeocoded:
		return AllocateStockArgs{OrderID: orderID}, nil
	case order.StatusReserved:
		return ChargePaymentArgs{OrderID: orderID}, nil
	case order.StatusReleasing:
		return ReleaseStockArgs{OrderID: orderID}, nil
	default:
		return nil, fmt.Errorf("no job moves an order out of %s", s)
	}
}

// sweepInterval is how often SweepStuckOrders looks for stuck orders.
const sweepInterval = 5 * time.Minute

// AddWorkers registers the order workers, all backed by orders.
func AddWorkers(workers *river.Workers, orders orderService) {
	river.AddWorker(workers, &GeocodeAddressWorker{orders: orders})
	river.AddWorker(workers, &AllocateStockWorker{orders: orders})
	river.AddWorker(workers, &ChargePaymentWorker{orders: orders})
	river.AddWorker(workers, &ReleaseStockWorker{orders: orders})
	river.AddWorker(workers, &SweepStuckOrdersWorker{orders: orders})
}

// PeriodicJobs returns the order jobs River runs on a schedule.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(sweepInterval), func() (river.JobArgs, *river.InsertOpts) {
			return SweepStuckOrdersArgs{}, nil
		}, nil),
	}
}

type GeocodeAddressWorker struct {
	river.WorkerDefaults[GeocodeAddressArgs]
	orders orderService
}

func (w *GeocodeAddressWorker) Work(ctx context.Context, job *river.Job[GeocodeAddressArgs]) error {
	return w.orders.GeocodeAddress(ctx, job.Args.OrderID)
}

type AllocateStockWorker struct {
	river.WorkerDefaults[AllocateStockArgs]
	orders orderService
}

func (w *AllocateStockWorker) Work(ctx context.Context, job *river.Job[AllocateStockArgs]) error {
	return w.orders.AllocateStock(ctx, job.Args.OrderID)
}

type ChargePaymentWorker struct {
	river.WorkerDefaults[ChargePaymentArgs]
	orders orderService
}

func (w *ChargePaymentWorker) Work(ctx context.Context, job *river.Job[ChargePaymentArgs]) error {
	return w.orders.ChargePayment(ctx, job.Args.OrderID)
}

type ReleaseStockWorker struct {
	river.WorkerDefaults[ReleaseStockArgs]
	orders orderService
}

func (w *ReleaseStockWorker) Work(ctx context.Context, job *river.Job[ReleaseStockArgs]) error {
	return w.orders.ReleaseStock(ctx, job.Args.OrderID)
}

// SweepStuckOrdersWorker finds the order jobs River discarded after their
// retries ran out, and retries the ones the order service asks for.
type SweepStuckOrdersWorker struct {
	river.WorkerDefaults[SweepStuckOrdersArgs]
	orders orderService
}

func (w *SweepStuckOrdersWorker) Work(ctx context.Context, job *river.Job[SweepStuckOrdersArgs]) error {
	client := river.ClientFromContext[pgx.Tx](ctx)
	kinds := make([]string, 0, len(waitingStatus))
	for kind := range waitingStatus {
		kinds = append(kinds, kind)
	}
	params := river.NewJobListParams().Kinds(kinds...).States(rivertype.JobStateDiscarded).First(100)
	for {
		res, err := client.JobList(ctx, params)
		if err != nil {
			return fmt.Errorf("listing discarded jobs: %w", err)
		}
		for _, stuck := range res.Jobs {
			if err := w.sweep(ctx, client, stuck); err != nil {
				return err
			}
		}
		if len(res.Jobs) == 0 || res.LastCursor == nil {
			return nil
		}
		params = params.After(res.LastCursor)
	}
}

// sweep hands one discarded job to the order service, and retries it when
// the service asks to.
func (w *SweepStuckOrdersWorker) sweep(ctx context.Context, client *river.Client[pgx.Tx], stuck *rivertype.JobRow) error {
	var args struct {
		OrderID uuid.UUID `json:"order_id"`
	}
	if err := json.Unmarshal(stuck.EncodedArgs, &args); err != nil {
		return fmt.Errorf("job %d: decoding args: %w", stuck.ID, err)
	}

	retry, err := w.orders.Discarded(ctx, args.OrderID, waitingStatus[stuck.Kind])
	if err != nil || !retry {
		return err
	}
	if _, err := client.JobRetry(ctx, stuck.ID); err != nil {
		return fmt.Errorf("order %s: retrying %s: %w", args.OrderID, stuck.Kind, err)
	}
	log.Printf("order %s: %s discarded after %d attempts, retrying it", args.OrderID, stuck.Kind, stuck.Attempt)
	return nil
}
