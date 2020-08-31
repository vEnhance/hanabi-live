package main

import (
	"sync"
)

var (
	// We keep track of all WebSocket sessions
	sessions      = make(map[int]*Session)
	sessionsMutex = sync.RWMutex{}

	// We only allow one user to connect or disconnect at the same time
	sessionConnectMutex = sync.Mutex{}

	// We keep track of all ongoing WebSocket messages/commands
	commandWaitGroup sync.WaitGroup
)
