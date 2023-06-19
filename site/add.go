package site

import (
	"context"

	"encore.dev/pubsub"
)

// AddParams are the parameters for adding a site to be monitored.
type AddParams struct {
	// URL is the URL of the site. If it doesn't contain a scheme
	// (like "http:" or "https:") it defaults to "https:".
	URL string `json:"url"`
}

// Add encore pubsub topic=site-add
var TopicSiteAdd = pubsub.NewTopic[*Site]("site-add", pubsub.TopicConfig{
	DeliveryGuarantee: pubsub.AtLeastOnce,
})

// Add adds a new site to the list of monitored websites.
//
// encore:api public method=POST path=/site
func (s *Service) Add(ctx context.Context, p *AddParams) (*Site, error) {
	site := &Site{URL: p.URL}
	if err := s.db.Create(site).Error; err != nil {
		return nil, err
	}
	if _, err := TopicSiteAdd.Publish(ctx, site); err != nil {
		return nil, err
	}
	return site, nil
}
