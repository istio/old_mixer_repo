package config

import (
	"istio.io/mixer/pkg/config/redis"
	"istio.io/mixer/pkg/config/store"
)

func StoreInventory() []store.RegisterFunc {
	return []store.RegisterFunc{
		redis.Register,
	}
}
