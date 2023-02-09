package powerflex

import (
	"sync"

	"github.com/dell/goscaleio"
	types "github.com/dell/goscaleio/types/v1"
)

// Client is the PowerFlex client required for the token getter and storage pool cache
// The token getter and storage pool cache use the same goscaelio client in different goroutines
// so we have to protect the client's api token from data races
type Client struct {
	client *goscaleio.Client
	mu     sync.Mutex // token lock
}

// NewClient returns a PowerFlex client to be used concurrently with the token getter and storage cache
func NewClient(c *goscaleio.Client) *Client {
	return &Client{
		client: c,
		mu:     sync.Mutex{},
	}
}

// Authenticate wraps the original Authenticate method
func (c *Client) Authenticate(configConnect *goscaleio.ConfigConnect) (goscaleio.Cluster, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.Authenticate(configConnect)
}

// GetToken wraps the original GetToken method
func (c *Client) GetToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.GetToken()
}

// SetToken wraps the original SetToken method
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.client.SetToken(token)
}

// FindStoragePool wraps the original FindStoragePool method
func (c *Client) FindStoragePool(id string, name string, href string, protectionDomain string) (*types.StoragePool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client.FindStoragePool(id, name, href, protectionDomain)
}
