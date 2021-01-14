package powerflex

type StoragePoolCache struct{}

func NewStoragePoolCache(addr string, size int) *StoragePoolCache {
	return nil
}

func (c *StoragePoolCache) GetStoragePoolNameByID(id string) (string, error) {
	return "", nil
}
