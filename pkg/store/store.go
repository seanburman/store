// Package store blah blah blah
package store

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
	"github.com/seanburman/kaw/cmd/api"
	"github.com/seanburman/kaw/cmd/gui"
	"github.com/seanburman/kaw/pkg/connection"
)

const serverStoreCache StoreKey = "server_store_cache"

var serverStore = NewStore("server_store")

func init() {
	cache, err := CreateStoreCache[api.Server](serverStore, serverStoreCache)
	if err != nil {
		log.Fatal(err)
	}
	cache.SetReducer(func(cfg ReducerConfig[api.Server]) interface{} {
		var data []struct {
			key interface{}
		}
		for _, server := range cfg.Data {
			for _, conn := range server.Item.Data.ConnectionPool.Connections() {
				log.Info(conn)
				data = append(data, struct {
					key interface{}
				}{
					key: "test",
				})
			}
		}
		return data
	})

	port := ":" + os.Getenv("KACHE_KROW_PORT")
	if port == ":" {
		port = ":8080"
	}
	serverStore.Serve(port, "/store")

	go func(feed chan map[time.Time]map[CacheKey]Item[api.Server]) {
		for item := range feed {
			fmt.Println(item)
		}
	}(cache.raw.feed)
}

type (
	Store struct {
		mu   sync.Mutex
		key  string
		data map[interface{}]interface{}
		keys []StoreKey
	}
	StoreKey string
)

func Kaw() {
	fmt.Println(`𓅩 KAW! Kaching At Will`)
	gui.ListenCommands()
}

func NewStore(key string) *Store {
	return &Store{
		key:  key,
		data: make(map[interface{}]interface{}),
	}
}

func (s *Store) Serve(port string, path string) error {
	path = strings.TrimSpace(path)

	cfg := api.NewConfig(port, path, s.key)
	server, err := api.NewServer(cfg)
	if err != nil {
		return err
	}

	caches, err := UseStoreCache[api.Server](serverStore, serverStoreCache)
	if err != nil {
		log.Fatal(err)
	}
	if err = caches.Save(server, s.key); err != nil {
		return err
	}

	server.SetOnNewConnection(func(c *connection.Connection) {
		var data []interface{}
		for _, v := range caches.GetAll() {
			for _, c := range v.Data.ConnectionPool.Connections() {
				data = append(data, c.Key)
			}
		}
		c.Publish(data)
	})

	server.ListenAndServe()
	return nil
}

func (s *Store) Keys() []StoreKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.keys
}

func CreateStoreCache[Cache interface{}](s *Store, key StoreKey) (*cache[Cache], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range s.keys {
		if v == key {
			return nil, fmt.Errorf("key %v already exists", key)
		}
	}
	s.keys = append(s.keys, key)
	cs := newCache[Cache]()
	s.data[key] = cs

	return cs, nil
}

func UseStoreCache[Cache interface{}](s *Store, key StoreKey) (*cache[Cache], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, ok := s.data[key]
	if !ok {
		return nil, fmt.Errorf("no data with key %v", key)
	}
	cache, ok := data.(*cache[Cache])
	if !ok {
		return nil, fmt.Errorf("invalid type for cache with key %v", key)
	}
	return cache, nil
}
