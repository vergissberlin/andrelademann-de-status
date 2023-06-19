package monitor

import (
	"context"
	"errors"

	"encore.app/site"
	"encore.dev/pubsub"
	"encore.dev/storage/sqldb"
)

// TransitionEvent describes a transition of a monitored site
// from up->down or from down->up.
type TransitionEvent struct {
	// Site is the monitored site in question.
	Site *site.Site `json:"site"`
	// Up specifies whether the site is now up or down (the new value).
	Up bool `json:"up"`
}

// TransitionTopic is a pubsub topic with transition events for when a monitored site
// transitions from up->down or from down->up.
var TransitionTopic = pubsub.NewTopic[*TransitionEvent]("uptime-transition", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})

// getPreviousMeasurement reports whether the given site was
// up or down in the previous measurement.
func getPreviousMeasurement(ctx context.Context, siteID int) (up bool, err error) {
	err = sqldb.QueryRow(ctx, `
		SELECT up FROM checks
		WHERE site_id = $1
		ORDER BY checked_at DESC
		LIMIT 1
	`, siteID).Scan(&up)

	if errors.Is(err, sqldb.ErrNoRows) {
		// There was no previous ping; treat this as if the site was up before
		return true, nil
	} else if err != nil {
		return false, err
	}
	return up, nil
}

func publishOnTransition(ctx context.Context, site *site.Site, isUp bool) error {
	wasUp, err := getPreviousMeasurement(ctx, site.ID)
	if err != nil {
		return err
	}
	if isUp == wasUp {
		// Nothing to do
		return nil
	}
	_, err = TransitionTopic.Publish(ctx, &TransitionEvent{
		Site: site,
		Up:   isUp,
	})
	return err
}

func check(ctx context.Context, site *site.Site) error {
	result, err := Ping(ctx, site.URL)
	if err != nil {
		return err
	}

	// Publish a Pub/Sub message if the site transitions
	// from up->down or from down->up.
	if err := publishOnTransition(ctx, site, result.Up); err != nil {
		return err
	}

	_, err = sqldb.Exec(ctx, `
		INSERT INTO checks (site_id, up, checked_at)
		VALUES ($1, $2, NOW())
	`, site.ID, result.Up)
	return err
}
