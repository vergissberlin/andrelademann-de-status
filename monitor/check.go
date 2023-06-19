package monitor

import (
	"context"

	"encore.app/site"
	"encore.dev/cron"
	"encore.dev/metrics"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
	"golang.org/x/sync/errgroup"
)

type Labels struct {
	Success bool
}

// Subscriptions
var _ = pubsub.NewSubscription(site.TopicSiteAdd, "site-add", pubsub.SubscriptionConfig[*site.Site]{
	Handler: check,
})

var Checked = metrics.NewCounterGroup[Labels, uint64]("check", metrics.CounterConfig{})

// Check checks a single site.
//
// encore:api public method=POST path=/check/:siteID
func Check(ctx context.Context, siteID int) error {
	var success bool
	Checked.With(Labels{Success: success}).Increment()
	site, err := site.Get(ctx, siteID)
	if err != nil {
		return err
	}
	result, err := Ping(ctx, site.URL)
	if err != nil {
		return err
	}
	_, err = sqldb.Exec(ctx, `
		INSERT INTO checks (site_id, up, checked_at)
		VALUES ($1, $2, NOW())
	`, site.ID, result.Up)
	return err
}

// CheckAll checks all sites.
//
// encore:api public method=POST path=/checkall
func CheckAll(ctx context.Context) error {
	// Get all the tracked sites.
	resp, err := site.List(ctx)
	if err != nil {
		return err
	}

	// Check up to 8 sites concurrently.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(8)
	for _, site := range resp.Sites {
		site := site // capture for closure
		g.Go(func() error {
			return Check(ctx, site.ID)
		})
	}

	return g.Wait()
}

// Check all tracked sites every 5 minutes.
var _ = cron.NewJob("check-all", cron.JobConfig{
	Title:    "Check all sites",
	Endpoint: CheckAll,
	Every:    5 * cron.Minute,
})
