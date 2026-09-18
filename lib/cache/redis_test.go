package cache

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"os"
	"reflect"
	"testing"
)

// redisTestOptions restituisce le opzioni del Redis di prova, letto da
// REDIS_ADDR (es. "127.0.0.1:6379"). Senza la variabile il test viene
// saltato: in CI il servizio lo avvia il workflow (REGOLE 6.1), altrove i
// test non devono dipendere da un Redis raggiungibile.
func redisTestOptions(tb testing.TB) *redis.Options {
	tb.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		tb.Skip("REDIS_ADDR non impostata: test Redis saltato")
	}
	return &redis.Options{Addr: addr}
}

func TestRedisSet(t *testing.T) {
	//rc := New("redis")
	rc := RedisCacheInit(redisTestOptions(t))
	err := rc.Set("123", "ddd", 0)
	if err != nil {
		fmt.Println(err.Error())
		t.Fatalf("写入失败")
	}
}

func TestRedisGet(t *testing.T) {
	rc := RedisCacheInit(redisTestOptions(t))
	err := rc.Set("123", "451156", 300)
	if err != nil {
		t.Fatalf("写入失败")
	}
	res := ""
	err = rc.Get("123", &res)
	if err != nil {
		t.Fatalf("读取失败")
	}
	fmt.Println("res", res)
}

func TestRedisGetJson(t *testing.T) {
	rc := RedisCacheInit(redisTestOptions(t))
	type r struct {
		Aa string `json:"a"`
		B  string `json:"c"`
	}
	old := &r{
		Aa: "ab", B: "cdc",
	}
	err := rc.Set("1233", old, 300)
	if err != nil {
		t.Fatalf("写入失败")
	}

	res := &r{}
	err2 := rc.Get("1233", res)
	if err2 != nil {
		t.Fatalf("读取失败")
	}
	if !reflect.DeepEqual(res, old) {
		t.Fatalf("读取错误")
	}
	fmt.Println(res, res.Aa)
}

func BenchmarkRSet(b *testing.B) {
	rc := RedisCacheInit(redisTestOptions(b))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc.Set("123", "{dsv}", 1000)
	}
}

func BenchmarkRGet(b *testing.B) {
	rc := RedisCacheInit(redisTestOptions(b))
	b.ResetTimer()
	v := ""
	for i := 0; i < b.N; i++ {
		rc.Get("123", &v)
	}
}
