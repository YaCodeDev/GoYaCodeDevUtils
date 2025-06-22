package cache

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

const YaMapLen = `[_____YaMapLen_____YA_/\_CODE_/\_DEV]`

type Memory struct {
	data   MemoryContainer
	mutex  sync.RWMutex
	ticker *time.Ticker
	done   chan bool
}

func NewMemory(data MemoryContainer, timeToClean time.Duration) *Memory {
	cache := Memory{
		data:   data,
		mutex:  sync.RWMutex{},
		ticker: time.NewTicker(timeToClean),
		done:   make(chan bool),
	}

	go cache.cleanup()

	return &cache
}

func (m *Memory) cleanup() {
	for {
		select {
		case <-m.ticker.C:
			m.mutex.Lock()

			for mainKey, mainValue := range m.data {
				for childKey, childValue := range mainValue {
					if childValue.isExpired() {
						delete(m.data[mainKey], childKey)

						if m.data.decrementLen(mainKey) == 0 {
							delete(m.data, mainKey)

							break
						}
					}
				}
			}

			m.mutex.Unlock()
		case <-m.done:
			return
		}
	}
}

func (m *Memory) Raw() MemoryContainer {
	return m.data
}

func (m *Memory) HSetEX(
	_ context.Context,
	mainKey string,
	childKey string,
	value string,
	ttl time.Duration,
) yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		childMap = make(map[string]*memoryCacheItem)

		m.data[mainKey] = childMap
	}

	childMap[childKey] = newMemoryCacheItemEX(value, time.Now().Add(ttl))

	m.data.incrementLen(mainKey)

	return nil
}

func (m *Memory) HGet(
	_ context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get map item")
	}

	value, err := childMap.get(childKey)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get map item")
	}

	return value, nil
}

func (m *Memory) HGetAll(
	_ context.Context,
	mainKey string,
) (map[string]string, yaerrors.Error) {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return nil, err.Wrap("[MEMORY] failed to get all map items")
	}

	result := make(map[string]string)

	for key, value := range childMap {
		if key != YaMapLen {
			result[key] = value.Value
		}
	}

	return result, nil
}

func (m *Memory) HGetDelSingle(
	_ context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get and delete item")
	}

	value, ok := childMap[childKey]
	if !ok {
		return "", yaerrors.FromString(http.StatusInternalServerError, "[MEMORY] childKey not found in childMap")
	}

	delete(childMap, childKey)

	m.data.decrementLen(mainKey)

	return value.Value, nil
}

func (m *Memory) HLen(
	_ context.Context,
	mainKey string,
) (int64, yaerrors.Error) {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	return int64(m.data.getLen(mainKey)), nil
}

func (m *Memory) HExist(
	_ context.Context,
	mainKey string,
	childKey string,
) (bool, yaerrors.Error) {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return false, err.Wrap("[MEMORY] failed to check exist")
	}

	return childMap.exist(childKey), nil
}

func (m *Memory) HDelSingle(
	_ context.Context,
	mainKey string,
	childKey string,
) yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return err.Wrap("[MEMORY] failed to delete item")
	}

	delete(childMap, childKey)

	m.data.decrementLen(mainKey)

	return nil
}

func (m *Memory) Ping() yaerrors.Error {
	return nil
}

func (m *Memory) Close() yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	for k := range m.data {
		delete(m.data, k)
	}

	m.done <- true

	return nil
}

type memoryCacheItem struct {
	Value     string
	ExpiresAt time.Time
	Endless   bool
}

func newMemoryCacheItem(value string) *memoryCacheItem {
	return &memoryCacheItem{
		Value:   value,
		Endless: true,
	}
}

func newMemoryCacheItemEX(
	value string,
	expiresAt time.Time,
) *memoryCacheItem {
	return &memoryCacheItem{
		Value:     value,
		ExpiresAt: expiresAt,
		Endless:   false,
	}
}

func (m *memoryCacheItem) isExpired() bool {
	return time.Now().After(m.ExpiresAt) && !m.Endless
}

type (
	MemoryContainer      map[string]childMemoryContainer
	childMemoryContainer map[string]*memoryCacheItem
)

func NewMemoryContainer() MemoryContainer {
	return make(MemoryContainer)
}

func (c childMemoryContainer) get(key string) (string, yaerrors.Error) {
	value, ok := c[key]
	if !ok {
		return "", yaerrors.FromString(
			http.StatusInternalServerError,
			fmt.Sprintf("[MEMORY] failed to get value in child map by `%s`", key),
		)
	}

	return value.Value, nil
}

func (c childMemoryContainer) exist(key string) bool {
	_, ok := c[key]

	return ok
}

func (m MemoryContainer) getLen(mainKey string) int {
	childMap, yaerr := m.getChildMap(mainKey)
	if yaerr != nil {
		return 0
	}

	value, ok := childMap[YaMapLen]
	if !ok {
		m[mainKey][YaMapLen] = newMemoryCacheItem("0")

		return 0
	}

	count, err := strconv.Atoi(value.Value)
	if err != nil {
		return 0
	}

	return count
}

func (m MemoryContainer) incrementLen(mainKey string) int {
	value := m.getLen(mainKey)

	value++

	m[mainKey][YaMapLen].Value = strconv.Itoa(value)

	return value
}

func (m MemoryContainer) decrementLen(mainKey string) int {
	value := m.getLen(mainKey)

	if value == 0 {
		return 0
	}

	value--

	m[mainKey][YaMapLen].Value = strconv.Itoa(value)

	return value
}

func (m MemoryContainer) getChildMap(mainKey string) (childMemoryContainer, yaerrors.Error) {
	childMap, ok := m[mainKey]
	if !ok {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			fmt.Sprintf("[MEMORY] failed to get main map by `%s`", mainKey),
		)
	}

	return childMap, nil
}
