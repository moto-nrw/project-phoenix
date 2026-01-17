package iot

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

var (
	// ErrNoEligibleCandidates indicates that an action had no applicable devices or students.
	ErrNoEligibleCandidates = errors.New("no eligible candidates for action")
)

const visitCooldown = 3 * time.Second

// Engine drives simulated events against the API based on cached discovery state.
type Engine struct {
	cfg     *Config
	client  *Client
	stateMu *sync.RWMutex
	states  map[string]*DeviceState

	metrics *EngineMetrics

	randMu sync.Mutex
	rand   *rand.Rand

	deviceConfigs map[string]DeviceConfig
}

// EngineMetrics tracks how many actions were executed.
type EngineMetrics struct {
	mu       sync.Mutex
	counts   map[ActionType]int64
	failures map[ActionType]int64
}

// NewEngine creates a new event engine instance.
func NewEngine(cfg *Config, client *Client, stateMu *sync.RWMutex, states map[string]*DeviceState) *Engine {
	configs := make(map[string]DeviceConfig, len(cfg.Devices))
	for _, device := range cfg.Devices {
		configs[device.DeviceID] = device
	}

	return &Engine{
		cfg:     cfg,
		client:  client,
		stateMu: stateMu,
		states:  states,
		metrics: &EngineMetrics{
			counts:   make(map[ActionType]int64),
			failures: make(map[ActionType]int64),
		},
		rand:          rand.New(rand.NewSource(time.Now().UnixNano())),
		deviceConfigs: configs,
	}
}

// Tick executes up to max_events_per_tick actions.
func (e *Engine) Tick(ctx context.Context) {
	maxEvents := e.cfg.Event.MaxEventsPerTick
	if maxEvents <= 0 {
		return
	}

	executed := make(map[ActionType]int)

	for i := 0; i < maxEvents; i++ {
		actionCfg, ok := e.selectAction()
		if !ok {
			return
		}

		if err := e.executeAction(ctx, actionCfg); err != nil {
			if errors.Is(err, ErrNoEligibleCandidates) {
				continue
			}
			e.metrics.recordFailure(actionCfg.Type)
			if logger.Logger != nil {
				logger.Logger.WithFields(map[string]interface{}{
					"action_type": string(actionCfg.Type),
					"error":       err.Error(),
				}).Warn("Action failed")
			}
		} else {
			e.metrics.recordSuccess(actionCfg.Type)
			executed[actionCfg.Type]++
		}
	}

	if len(executed) > 0 && logger.Logger != nil {
		parts := make([]string, 0, len(executed))
		for action, count := range executed {
			parts = append(parts, fmt.Sprintf("%s=%d", action, count))
		}
		logger.Logger.WithField("summary", strings.Join(parts, " ")).Debug("Tick summary")
	}
}

func (e *Engine) selectAction() (ActionConfig, bool) {
	candidates := make([]ActionConfig, 0, len(e.cfg.Event.Actions))
	var totalWeight float64
	for _, action := range e.cfg.Event.Actions {
		if action.Disabled {
			continue
		}
		if action.Weight <= 0 {
			continue
		}
		candidates = append(candidates, action)
		totalWeight += action.Weight
	}

	if len(candidates) == 0 || totalWeight <= 0 {
		return ActionConfig{}, false
	}

	e.randMu.Lock()
	r := e.rand.Float64() * totalWeight
	e.randMu.Unlock()

	var cumulative float64
	for _, action := range candidates {
		cumulative += action.Weight
		if r < cumulative {
			return action, true
		}
	}

	// Fallback to last candidate (should not typically happen due to floating point rounding)
	return candidates[len(candidates)-1], true
}

func (e *Engine) executeAction(ctx context.Context, action ActionConfig) error {
	switch action.Type {
	case ActionCheckIn:
		return e.executeCheckIn(ctx, action)
	case ActionCheckOut:
		return e.executeCheckOut(ctx, action)
	case ActionSchulhofHop:
		return e.executeSchulhofHop(ctx, action)
	case ActionAttendanceToggle:
		return e.executeAttendanceToggle(ctx, action)
	case ActionSupervisorSwap:
		return e.executeSupervisorSwap(ctx, action)
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
}

func (m *EngineMetrics) recordSuccess(action ActionType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counts[action]++
}

func (m *EngineMetrics) recordFailure(action ActionType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failures[action]++
}

// errDeviceNotConfigured returns an error for unconfigured devices.
func errDeviceNotConfigured(deviceID string) error {
	return fmt.Errorf("device %s not configured", deviceID)
}
