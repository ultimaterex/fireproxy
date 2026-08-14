package collect

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// GoRedis adapts go-redis to RedisReader.
type GoRedis struct {
	Client *redis.Client
}

func NewGoRedis(addr string) *GoRedis {
	return &GoRedis{Client: redis.NewClient(&redis.Options{Addr: addr})}
}

func (g *GoRedis) HGet(key, field string) (string, error) {
	return g.Client.HGet(context.Background(), key, field).Result()
}

func (g *GoRedis) Get(key string) (string, error) {
	return g.Client.Get(context.Background(), key).Result()
}

func (g *GoRedis) HGetAll(key string) (map[string]string, error) {
	return g.Client.HGetAll(context.Background(), key).Result()
}

func (g *GoRedis) Scan(pattern string, count int64) ([]string, error) {
	ctx := context.Background()
	var (
		cursor uint64
		out    []string
	)
	if count <= 0 {
		count = 100
	}
	for {
		keys, next, err := g.Client.Scan(ctx, cursor, pattern, count).Result()
		if err != nil {
			return out, err
		}
		out = append(out, keys...)
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return out, nil
}

func (g *GoRedis) HMGet(key string, fields ...string) ([]string, error) {
	if len(fields) == 0 {
		return nil, nil
	}
	raw, err := g.Client.HMGet(context.Background(), key, fields...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]string, len(raw))
	for i, v := range raw {
		if v == nil {
			continue
		}
		switch t := v.(type) {
		case string:
			out[i] = t
		default:
			out[i] = ""
		}
	}
	return out, nil
}

func (g *GoRedis) ZCard(key string) (int64, error) {
	n, err := g.Client.ZCard(context.Background(), key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	return n, err
}

func (g *GoRedis) ZRevRangeByScore(key, max, min string, offset, count int64) ([]ZMember, error) {
	vals, err := g.Client.ZRevRangeByScoreWithScores(context.Background(), key, &redis.ZRangeBy{
		Min:    min,
		Max:    max,
		Offset: offset,
		Count:  count,
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]ZMember, 0, len(vals))
	for _, z := range vals {
		out = append(out, ZMember{Member: fmtRedisMember(z.Member), Score: z.Score})
	}
	return out, nil
}

func fmtRedisMember(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}

func (g *GoRedis) Close() error {
	return g.Client.Close()
}
