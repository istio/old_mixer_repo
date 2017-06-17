package config

import (
	"istio.io/mixer/pkg/config/redis"
	"istio.io/mixer/pkg/config/store"
)

// StoreInventory returns the inventory of store backends.
func StoreInventory() []store.RegisterFunc {
	return []store.RegisterFunc{
		redis.Register,
	}
}
