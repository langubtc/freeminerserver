package storage

import (
	"log"
	"testing"
)

var r *RedisClient


func TestRedisClient_GetAllWorkShares(t *testing.T) {
	r = NewRedisClient(&Config{Endpoint: "127.0.0.1:6379"}, "eth")
	result :=r.GetAllWorkShares()
	log.Println(result)
}
func TestRedisClient_ClearWorkShare(t *testing.T) {
	r = NewRedisClient(&Config{Endpoint: "127.0.0.1:6379"}, "eth")
	result :=r.ClearWorkShare("z3070")
	log.Println(result)
}
func TestRedisClient_ClearAllWorkShare(t *testing.T) {
	r = NewRedisClient(&Config{Endpoint: "127.0.0.1:6379"}, "eth")
	result :=r.ClearAllWorkShare()
	log.Println(result)
}
func TestRedisClient_WriteShare(t *testing.T) {
	r = NewRedisClient(&Config{Endpoint: "127.0.0.1:6379"}, "eth")
	r.WriteShareId( r.client.Multi() ,"z3071")
	r.WriteShareId( r.client.Multi() ,"z3022")
}