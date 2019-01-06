package policy

import (
	"fmt"
	"time"

	"github.com/golangci/golangci-api/internal/shared/cache"
	"github.com/golangci/golangci-api/internal/shared/config"
	"github.com/golangci/golangci-api/internal/shared/providers/provider"
	"github.com/golangci/golangci-api/pkg/api/request"
	"github.com/pkg/errors"
)

type orgFetcher struct {
	cache cache.Cache
	cfg   config.Config
}

func (of orgFetcher) fetch(rc *request.AuthorizedContext, p provider.Provider, orgName string) (*provider.Org, error) {
	providerOrg, fromCache, err := of.fetchCached(rc, true, p, orgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch org from cached provider")
	}

	if !providerOrg.IsAdmin && fromCache { // user may have become an admin recently, refetch
		rc.Log.Infof("User isn't an admin in the org %s from cache, refetch it from the provider without cache", orgName)

		providerOrg, _, err = of.fetchCached(rc, false, p, orgName)
		if err != nil {
			return nil, errors.Wrap(err, "failed to fetch org from not cached provider")
		}
	}

	return providerOrg, nil
}

func (of orgFetcher) fetchCached(rc *request.AuthorizedContext, useCache bool,
	p provider.Provider, orgName string) (*provider.Org, bool, error) {

	key := fmt.Sprintf("orgs/%s/fetch?user_id=%d&org_name=%s&v=1", p.Name(), rc.Auth.UserID, orgName)

	var org *provider.Org
	if useCache {
		if err := of.cache.Get(key, &org); err != nil {
			rc.Log.Warnf("Can't fetch org from cache by key %s: %s", key, err)
			providerOrg, fetchErr := of.fetchFromProvider(rc, p, orgName)
			return providerOrg, false, fetchErr
		}

		if org != nil {
			rc.Log.Infof("Returning org(%d) from cache", org.ID)
			return org, true, nil
		}

		rc.Log.Infof("No org in cache, fetching them from provider...")
	} else {
		rc.Log.Infof("Don't lookup org in cache, refreshing org from provider...")
	}

	var err error
	org, err = of.fetchFromProvider(rc, p, orgName)
	if err != nil {
		return nil, false, err
	}

	cacheTTL := of.cfg.GetDuration("ORG_CACHE_TTL", time.Hour*24*7)
	if err = of.cache.Set(key, cacheTTL, org); err != nil {
		rc.Log.Warnf("Can't save org to cache by key %s: %s", key, err)
	}

	return org, false, nil
}

func (of orgFetcher) fetchFromProvider(rc *request.AuthorizedContext, p provider.Provider, orgName string) (*provider.Org, error) {
	org, err := p.GetOrgByName(rc.Ctx, orgName)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch org from provider by name %s", orgName)
	}

	return org, nil
}