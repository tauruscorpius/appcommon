package RedisCluster

import (
	"context"
	"errors"
	"github.com/go-redis/redis/v8"
	"github.com/tauruscorpius/appcommon/Log"
	"sync"
	"time"
)

var (
	redisBase *RedisClusterBase
	onceSvc   sync.Once
)

func GetRedisBase() *RedisClusterBase {
	onceSvc.Do(func() {
		redisBase = &RedisClusterBase{}
	})
	return redisBase
}

type RedisClusterPool struct {
	redisUser     string
	redisPassword string
	pool          []string
}

func (t *RedisClusterPool) Add(host, port string) {
	t.pool = append(t.pool, host+":"+port)
}

type RedisClusterBase struct {
	cluster *redis.ClusterClient
}

func (t RedisClusterBase) Get() *redis.ClusterClient {
	return t.cluster
}

func (t *RedisClusterBase) ClusterInit(redisPool *RedisClusterPool) error {
	if redisPool == nil {
		t.cluster = nil
		return nil
	}
	Log.Criticalf("Redis Pool : %+v", *redisPool)
	option := &redis.ClusterOptions{
		Addrs:              redisPool.pool,
		DialTimeout:        5 * time.Second,
		ReadTimeout:        1 * time.Second,
		WriteTimeout:       1 * time.Second,
		PoolSize:           500,
		PoolTimeout:        30 * time.Second,
		IdleTimeout:        time.Minute,
		IdleCheckFrequency: 1 * time.Second,
	}
	if redisPool.redisPassword != "" {
		if redisPool.redisUser != "" {
			option.Username = redisPool.redisUser
		}
		option.Password = redisPool.redisPassword
	}
	cluster := redis.NewClusterClient(option)
	if cluster == nil {
		return errors.New("cluster new")
	}
	ctx := context.Background()
	ctxTimeout, _ := context.WithTimeout(ctx, time.Second)
	pong, err := cluster.Ping(ctxTimeout).Result()
	Log.Infof("redis pong status[%v]\n", pong)
	if err != nil {
		return err
	}
	t.cluster = cluster
	return nil
}
