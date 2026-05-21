package redisclient

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type DeviceState struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Status   string `json:"status"`
	LastSeen int64  `json:"last_seen"`
	IsRoot   bool   `json:"is_root"`
}

type Client struct {
	Rdb      *redis.Client
	Ctx      context.Context
	DeviceID string
	Hostname string
	IsRoot   bool
}

func NewClient(ctx context.Context, addr, password, deviceID, hostname string, isRoot bool) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	return &Client{Rdb: rdb, Ctx: ctx, DeviceID: deviceID, Hostname: hostname, IsRoot: isRoot}
}

func (c *Client) UpdateState(status string) {
	state := DeviceState{
		ID:       c.DeviceID,
		Hostname: c.Hostname,
		Status:   status,
		LastSeen: time.Now().Unix(),
		IsRoot:   c.IsRoot,
	}
	data, _ := json.Marshal(state)
	c.Rdb.HSet(c.Ctx, "devices_state", c.DeviceID, string(data))

	heartbeatKey := "heartbeat:" + c.DeviceID
	if status == "Online" {
		c.Rdb.Set(c.Ctx, heartbeatKey, "alive", 60*time.Second)
	} else {
		c.Rdb.Del(c.Ctx, heartbeatKey)
	}
}

func (c *Client) PublishResult(result string) {
	c.Rdb.Publish(c.Ctx, "results:"+c.DeviceID, result)
}

func (c *Client) SubscribeCommands() (<-chan *redis.Message, error) {
	pubsub := c.Rdb.Subscribe(c.Ctx, "commands:"+c.DeviceID, "commands:all")

	_, err := pubsub.Receive(c.Ctx)
	if err != nil {
		return nil, err
	}

	return pubsub.Channel(), nil
}
