package powerflex

import (
	"fmt"

	"github.com/dell/goscaleio"
	lru "github.com/hashicorp/golang-lru"
)

type StoragePoolCache struct {
	client *goscaleio.Client
	cache  *lru.Cache
}

type StoragePoolCacheConfig struct {
	PowerFlexClient *goscaleio.Client
	Size            int
}

func NewStoragePoolCache(c StoragePoolCacheConfig) (*StoragePoolCache, error) {
	cache, err := lru.New(c.Size)
	if err != nil {
		return nil, err
	}

	return &StoragePoolCache{
		client: c.PowerFlexClient,
		cache:  cache,
	}, nil
}

func (c *StoragePoolCache) GetStoragePoolNameByID(id string) (string, error) {
	if v, ok := c.cache.Get(id); ok {
		name, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("cache value %T is not a string", v)
		}
		return name, nil
	}

	pool, err := c.client.FindStoragePool(id, "", "")
	if err != nil {
		return "", nil
	}

	c.cache.Add(id, pool.Name)

	return pool.Name, nil
}
