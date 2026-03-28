package handler

import (
	"sync"
	"time"
)

type TimerManager struct {
	timers map[string]*time.Timer
	mu     sync.RWMutex
	endFn  func(roomCode string)
}

func NewTimerManager(endFn func(roomCode string)) *TimerManager {
	return &TimerManager{
		timers: make(map[string]*time.Timer),
		endFn:  endFn,
	}
}

func (tm *TimerManager) Start(roomCode string, seconds int) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if existing, ok := tm.timers[roomCode]; ok {
		existing.Stop()
	}

	duration := time.Duration(seconds) * time.Second
	tm.timers[roomCode] = time.AfterFunc(duration, func() {
		tm.mu.Lock()
		delete(tm.timers, roomCode)
		tm.mu.Unlock()
		if tm.endFn != nil {
			tm.endFn(roomCode)
		}
	})
}

func (tm *TimerManager) Cancel(roomCode string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if timer, ok := tm.timers[roomCode]; ok {
		timer.Stop()
		delete(tm.timers, roomCode)
	}
}

func (tm *TimerManager) CancelAll() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	for _, timer := range tm.timers {
		timer.Stop()
	}
	tm.timers = make(map[string]*time.Timer)
}
