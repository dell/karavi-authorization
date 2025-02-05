package storage

import (
	"context"
	"sync"

	"github.com/dell/goscaleio"
	types "github.com/dell/goscaleio/types/v1"
	"golang.org/x/sync/semaphore"
)

type rateLimitedPowerFlexClient struct {
	client *goscaleio.Client
	sem    *semaphore.Weighted
	lock   sync.Mutex
}

func newRateLimitedPowerFlexClient(client *goscaleio.Client, semaphore *semaphore.Weighted) *rateLimitedPowerFlexClient {
	return &rateLimitedPowerFlexClient{
		client: client,
		sem:    semaphore,
		lock:   sync.Mutex{},
	}
}

func (c *rateLimitedPowerFlexClient) GetVolume(ctx context.Context, volumehref string, volumeid string, ancestorvolumeid string, volumename string, getSnapshots bool) ([]*types.Volume, error) {
	err := c.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	defer c.sem.Release(1)

	c.lock.Lock()
	defer c.lock.Unlock()

	return c.client.GetVolume(volumehref, volumeid, ancestorvolumeid, volumename, getSnapshots)
}

func (c *rateLimitedPowerFlexClient) FindStoragePool(ctx context.Context, id string, name string, href string, protectionDomain string) (*types.StoragePool, error) {
	err := c.sem.Acquire(ctx, 1)
	if err != nil {
		return nil, err
	}
	defer c.sem.Release(1)

	c.lock.Lock()
	defer c.lock.Unlock()

	return c.client.FindStoragePool(id, name, href, protectionDomain)
}
