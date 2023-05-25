package base

import (
	"github.com/go-redis/redis/v8"
	"github.com/tauruscorpius/appcommon/RedisCluster"
)

type RedisImplFunc interface {
	KeyMaker(interface{}) string
	NewNode() interface{}
	HashMaker(interface{}) uint64
	UpdateTime(interface{})
}

type RedisImplBase struct {
	cluster  RedisCluster.RedisClusterBase
	ImplFunc RedisImplFunc
}

func (t *RedisImplBase) Init(base *RedisCluster.RedisClusterBase, cbImpFunc RedisImplFunc) bool {
	t.cluster = *base
	t.ImplFunc = cbImpFunc

	return true
}

func (t *RedisImplBase) GetClusterClient() *redis.ClusterClient {
	return t.cluster.Get()
}
