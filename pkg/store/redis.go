package store

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	redisOne  sync.Once
	redisInst *redis.Client
)

type RedisOption func(*Redis)

// RedisOptions to connect
type Connection struct {
	Addr     string
	Password string
	DB       int
}

// Redis connection
type Redis struct {
	Conn *Connection
}

func WithRedisAddr(a string) RedisOption {
	return func(r *Redis) {
		r.Conn.Addr = a
	}
}

func WithRedisConn(cfg *Connection) RedisOption {
	return func(r *Redis) {
		r.Conn = cfg
	}
}

func WithDefaults() RedisOption {
	return func(r *Redis) {
		r.Conn = &Connection{
			Addr: "localhost:6379",
		}
	}
}

// Connect to redis
func (r *Redis) Connect() {
	log.Println("Connecting to Redis Database")
	redisInst = redis.NewClient(&redis.Options{
		Addr:     r.Conn.Addr,
		Password: r.Conn.Password,
		DB:       r.Conn.DB,
	})
	log.Println("Connected")
}

// GetInstance a singleton instance of redis
func (r *Redis) GetInstance() *redis.Client {
	redisOne.Do(r.Connect)
	return redisInst
}

// NewRedis create a redis store, this will be a wrapper of Redis library
// to get a unique instance of redis.
func NewRedis(opts ...RedisOption) *Redis {
	defaultConn := &Connection{
		Addr: "localhost:6379",
	}

	r := &Redis{
		Conn: defaultConn,
	}

	for _, opt := range opts {
		opt(r)
	}

	return r
}

type ProducerOption func(*Producer)

func WithRedis(r *Redis) ProducerOption {
	return func(p *Producer) {
		p.RDB = r
	}
}

func WithStream(s string) ProducerOption {
	return func(p *Producer) {
		p.Stream = s
	}
}

func WithMaxLen(m int64) ProducerOption {
	return func(p *Producer) {
		p.MaxLenApprox = m
	}
}

func NewProducer(opts ...ProducerOption) *Producer {
	p := &Producer{
		Stream:       "RAW.DEF",
		MaxLenApprox: 10,
		Namespace:    "RSTRM.",
	}

	for _, opt := range opts {
		opt(p)
	}
	return p
}

func DefaultProducer() *Producer {

	r := NewRedis()
	p := &Producer{
		RDB:          r,
		Stream:       "RAW.DEF",
		MaxLenApprox: 10,
		Namespace:    "RSTRM.",
	}
	return p
}

//Producer redis streams producer
type Producer struct {
	RDB          *Redis
	Stream       string
	MaxLenApprox int64
	Namespace    string
}

// Send Xadd command wrapper
func (p *Producer) Send(ctx context.Context, values interface{}) error {
	rdb := p.RDB.GetInstance()
	args := &redis.XAddArgs{
		Stream:       p.Stream,
		MaxLenApprox: p.MaxLenApprox,
		ID:           "*",
		Values:       values,
	}
	_, err := rdb.XAdd(ctx, args).Result()
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

/*type Message struct {
	Stream string
	MaxLen int64
	ID string
	Values interface{}
}*/

func (p *Producer) SendTo(ctx context.Context, stream string, values interface{}) error {
	rdb := p.RDB.GetInstance()
	args := &redis.XAddArgs{
		Stream:       stream,
		MaxLenApprox: p.MaxLenApprox,
		ID:           "*",
		Values:       values,
	}
	res, err := rdb.XAdd(ctx, args).Result()
	if err != nil {
		log.Println(err)
		return err
	}
	fmt.Println(res)
	return nil
}

// CreateGroup Create a redis stream group
func (p *Producer) CreateGroup(ctx context.Context, stream, group, start string) error {

	rdb := p.RDB.GetInstance()
	_, err := rdb.XGroupCreate(ctx, stream, group, start).Result()
	if err != nil {
		log.Println(err)
		return err
	}
	return nil
}

// Consumer redis stream consumer
type Consumer struct {
	RDB *Redis
	ID  string
	// Streams []string
	Group string
}

// GroupOpts ReadGroup options
type GroupOpts struct {
	Streams []string
	Count   int64
	Block   time.Duration
	Ack     bool
}

// ReadMessage function to read message from readis stream
type ReadMessage func(string, string, interface{}) error

// ReadGroup read messages from redis stream
// This function receives a function of type ReadMessage.
// Also, if the functions ends without errors, the message will be marked with
// an ACK.
func (c *Consumer) ReadGroup(ctx context.Context, opts *GroupOpts, f ReadMessage) {

	rdb := c.RDB.GetInstance()
	args := &redis.XReadGroupArgs{
		Group:    c.Group,
		Streams:  opts.Streams,
		Consumer: c.ID,
		Count:    opts.Count,
		Block:    opts.Block,
		NoAck:    opts.Ack,
	}
	streams, err := rdb.XReadGroup(ctx, args).Result()
	if err != nil {
		log.Println(err)
	}
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			err := f(stream.Stream, msg.ID, msg.Values)
			if err != nil {
				log.Println(msg)
				log.Println(err)
			} else {
				rdb.XAck(ctx, stream.Stream, c.Group, msg.ID)
			}

			// fmt.Printf("Msg from stream %s: %v\n", stream, msg)
		}

	}

}
