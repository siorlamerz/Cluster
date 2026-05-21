package redisclient

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
)

type Manager struct {
	Rdb *redis.Client
	Ctx context.Context
}

func NewManager(ctx context.Context, addr, password string) *Manager {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	return &Manager{Rdb: rdb, Ctx: ctx}
}

// возвращает список всех зарегистрированных устройств и проверяет их статус активности
func (m *Manager) GetDevices() ([]DeviceState, error) {
	statesMap, err := m.Rdb.HGetAll(m.Ctx, "devices_state").Result()
	if err != nil {
		return nil, err
	}

	var devices []DeviceState
	for _, val := range statesMap {
		var state DeviceState
		if err := json.Unmarshal([]byte(val), &state); err != nil {
			continue
		}

		if state.ID == "" {
			continue
		}

		heartbeatKey := "heartbeat:" + state.ID
		exists, err := m.Rdb.Exists(m.Ctx, heartbeatKey).Result()
		if err == nil && exists > 0 {
			state.Status = "Online"
		} else {
			state.Status = "Offline"
		}

		devices = append(devices, state)
	}
	return devices, nil
}

// отправляет команду конкретному устройству
func (m *Manager) SendCommand(deviceID, command string) error {
	channel := "commands:" + deviceID
	return m.Rdb.Publish(m.Ctx, channel, command).Err()
}

// отправляет команду всем устройствам одновременно
func (m *Manager) SendCommandAll(command string) error {
	return m.Rdb.Publish(m.Ctx, "commands:all", command).Err()
}

// подписывается на канал результатов одного устройства или всех сразу
func (m *Manager) ListenToResults(deviceID string) (<-chan *redis.Message, *redis.PubSub, error) {
	var pubsub *redis.PubSub
	if deviceID == "all" {
		pubsub = m.Rdb.PSubscribe(m.Ctx, "results:*")
	} else {
		pubsub = m.Rdb.Subscribe(m.Ctx, "results:"+deviceID)
	}

	_, err := pubsub.Receive(m.Ctx)
	if err != nil {
		return nil, nil, err
	}

	return pubsub.Channel(), pubsub, nil
}

func (m *Manager) PruneDevices() (int, error) {
	statesMap, err := m.Rdb.HGetAll(m.Ctx, "devices_state").Result()
	if err != nil {
		return 0, err
	}

	removedCount := 0
	for deviceID := range statesMap {
		heartbeatKey := "heartbeat:" + deviceID
		exists, err := m.Rdb.Exists(m.Ctx, heartbeatKey).Result()

		// Если ключа нет или произошла ошибка то устройство мертво нахуй
		if err != nil || exists == 0 {
			// делитаем
			m.Rdb.HDel(m.Ctx, "devices_state", deviceID)
			removedCount++
		}
	}
	return removedCount, nil
}
