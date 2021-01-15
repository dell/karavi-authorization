package powerflex

import (
	"fmt"

	"github.com/dell/goscaleio"
	lru "github.com/hashicorp/golang-lru"
)

type StoragePoolCache struct {
	client    *goscaleio.Client
	nameCache *lru.Cache
}

type StoragePoolCacheConfig struct {
	PowerFlexClient *goscaleio.Client
	Size            int
}

func NewStoragePoolCache(c StoragePoolCacheConfig) (*StoragePoolCache, error) {
	nameCache, err := lru.New(c.Size)
	if err != nil {
		return nil, err
	}

	return &StoragePoolCache{
		client:    c.PowerFlexClient,
		nameCache: nameCache,
	}, nil
}

func (c *StoragePoolCache) GetStoragePoolNameByID(id string) (string, error) {
	if v, ok := c.nameCache.Get(id); ok {
		name, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("cache value %T is not a string", v)
		}
		return name, nil
	}

	pool, err := c.client.FindStoragePool(id, "", "")
	if err != nil {
		return "", err
	}

	c.nameCache.Add(id, pool.Name)

	return pool.Name, nil
}
