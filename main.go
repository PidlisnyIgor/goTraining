package main

import (
	"context"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	"log"
	"net/http"
	"strconv"
)

type Item struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

var db *RedisDB
var ctx = context.Background()

func main() {
	var err error
	db, err = NewRedisDB("localhost:6379", "", 0)
	if err != nil {
		log.Fatalf("Could not connect to Redis: %v", err)
	}

	http.HandleFunc("/items", handleItems)
	http.HandleFunc("/items/", handleItem)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleItems(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		allItems, err := db.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(allItems)
	case "POST":
		var newItem Item
		if err := json.NewDecoder(r.Body).Decode(&newItem); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := db.Create(&newItem); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(newItem)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleItem(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/items/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		item, err := db.Read(id)
		if err != nil {
			http.Error(w, "Item not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(item)
	case "PUT":
		var updatedItem Item
		if err := json.NewDecoder(r.Body).Decode(&updatedItem); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updatedItem.ID = id
		if err := db.Update(id, &updatedItem); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(updatedItem)
	case "DELETE":
		if err := db.Delete(id); err != nil {
			http.Error(w, "Failed to delete item", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type RedisDB struct {
	client *redis.Client
}

func NewRedisDB(addr, password string, db int) (*RedisDB, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisDB{client: client}, nil
}

func (r *RedisDB) Create(item *Item) error {
	id, err := r.NextID()
	if err != nil {
		return err
	}
	item.ID = id
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, "item:"+strconv.Itoa(item.ID), data, 0).Err()
}

func (r *RedisDB) Read(id int) (*Item, error) {
	data, err := r.client.Get(ctx, "item:"+strconv.Itoa(id)).Bytes()
	if err != nil {
		return nil, err
	}
	var item Item
	err = json.Unmarshal(data, &item)
	return &item, err
}

func (r *RedisDB) Update(id int, item *Item) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, "item:"+strconv.Itoa(id), data, 0).Err()
}

func (r *RedisDB) Delete(id int) error {
	return r.client.Del(ctx, "item:"+strconv.Itoa(id)).Err()
}

func (r *RedisDB) List() ([]*Item, error) {
	keys, err := r.client.Keys(ctx, "item:*").Result()
	if err != nil {
		return nil, err
	}

	var items []*Item
	for _, key := range keys {
		itemData, err := r.client.Get(ctx, key).Bytes()
		if err != nil {
			return nil, err
		}
		var item Item
		if err := json.Unmarshal(itemData, &item); err != nil {
			return nil, err
		}
		items = append(items, &item)
	}
	return items, nil
}

func (r *RedisDB) NextID() (int, error) {
	id, err := r.client.Incr(ctx, "itemID").Result()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}
