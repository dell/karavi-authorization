// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package powerflex

import (
	"context"
	"fmt"
	"sync"

	"github.com/dell/goscaleio"
	lru "github.com/hashicorp/golang-lru"
	"go.opentelemetry.io/otel/trace"
)

// StoragePoolCache is a least recently used cache of PowerFlex storage pool names
type StoragePoolCache struct {
	client *goscaleio.Client
	cache  *lru.Cache
	mu     sync.Mutex
}

// NewStoragePoolCache creates a new StoragePoolCache
// It requires a goscaelio client and a cache size
func NewStoragePoolCache(client *goscaleio.Client, cacheSize int) (*StoragePoolCache, error) {
	if client == nil {
		return nil, fmt.Errorf("goscaleio client is required")
	}

	if cacheSize < 1 {
		return nil, fmt.Errorf("cache size must be at least one")
	}

	cache, err := lru.New(cacheSize)
	if err != nil {
		return nil, err
	}

	return &StoragePoolCache{
		client: client,
		cache:  cache,
	}, nil
}

// PowerFlexTokenGetter manages and retains a valid token for a PowerFlex
type LoginTokenGetter interface {
	GetToken(context.Context) (string, error)
}

// GetStoragePoolNameByID returns the storage pool's name from the cache via the storage pool's ID
func (c *StoragePoolCache) GetStoragePoolNameByID(ctx context.Context, tokenGetter LoginTokenGetter, id string) (string, error) {
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("").Start(ctx, "GetStoragePoolNameByID")
	defer span.End()

	c.mu.Lock()
	defer c.mu.Unlock()

	if v, ok := c.cache.Get(id); ok {
		name, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("cache value %T is not a string", v)
		}
		return name, nil
	}

	token, err := tokenGetter.GetToken(ctx)
	if err != nil {
		return "", err
	}

	c.client.SetToken(token)

	pool, err := c.client.FindStoragePool(id, "", "")
	if err != nil {
		return "", err
	}

	c.cache.Add(id, pool.Name)

	return pool.Name, nil
}
