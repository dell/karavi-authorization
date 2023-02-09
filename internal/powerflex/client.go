package powerflex

import (
	"sync"

	"github.com/dell/goscaleio"
	types "github.com/dell/goscaleio/types/v1"
)

// client is the PowerFlex client required for the token getter and storage pool cache
// The token getter and storage pool cache use the same goscaelio client in different goroutines
// so we have to protect the client's api token from data races
type client struct {
	client *goscaleio.Client
	mu     sync.Mutex // token lock
}

// NewClient returns a PowerFlex client to be used concurrently with the token getter and storage cache
func NewClient(c *goscaleio.Client) *client {
	return &client{
		client: c,
		mu:     sync.Mutex{},
	}
}

func (c *client) Authenticate(configConnect *goscaleio.ConfigConnect) (goscaleio.Cluster, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.Authenticate(configConnect)
}

func (c *client) GetToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.GetToken()
}

func (c *client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client.SetToken(token)
}

func (c *client) FindStoragePool(id string, name string, href string, protectionDomain string) (*types.StoragePool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.FindStoragePool(id, name, href, protectionDomain)
}
