// General purpose session functions

package main

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type Session struct {
	Conn   *websocket.Conn
	Mutex  *sync.RWMutex
	Closed bool

	SessionID          uint64
	UserID             int
	Username           string
	Muted              bool
	Status             int
	TableID            uint64
	Friends            map[int]struct{}
	ReverseFriends     map[int]struct{}
	Hyphenated         bool
	Inactive           bool
	FakeUser           bool
	RateLimitAllowance float64
	RateLimitLastCheck time.Time
	Banned             bool
}

var (
	// The counter is atomically incremented before assignment,
	// so the first session ID will be 1 and will increase from there
	sessionIDCounter uint64 = 0
)

func NewSession() *Session {
	// Specify the default values used for both real sessions and fake sessions
	return &Session{
		SessionID:          atomic.AddUint64(&sessionIDCounter, 1),
		UserID:             -1,
		Status:             StatusLobby, // By default, new users are in the lobby
		Friends:            make(map[int]struct{}),
		ReverseFriends:     make(map[int]struct{}),
		RateLimitAllowance: RateLimitRate,
		RateLimitLastCheck: time.Now(),
	}
}

// NewFakeSession prepares a "fake" user session that will be used for game emulation
func NewFakeSession(id int, name string) *Session {
	s := NewSession()
	s.UserID = id
	s.Username = name
	s.FakeUser = true

	return s
}

// Emit sends a message to a client using the Golem-style protocol described above
func (s *Session) Emit(command string, d interface{}) {
	if s == nil || s.Conn == nil {
		return
	}

	// Convert the data to JSON
	var ds string
	if dj, err := json.Marshal(d); err != nil {
		logger.Error("Failed to marshal data when writing to a WebSocket session:", err)
		return
	} else {
		ds = string(dj)
	}

	// Send the message as bytes
	msg := command + " " + ds
	bytes := []byte(msg)
	if err := s.Conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		// Can this can routinely fail if the session is closed?
		logger.Error("Failed to write to the session of user \""+s.Username+"\":", err)
		return
	}
}

func (s *Session) Close() {
	if s.Closed {
		return
	}


		s.Mutex.Lock()
		s.open = false
		s.conn.Close()
		close(s.output)
		s.Mutex.Unlock()
	}
}

func (s *Session) Warning(message string) {
	// Specify a default warning message
	if message == "" {
		message = DefaultErrorMsg
	}

	logger.Info("Warning - " + message + " - " + s.Username)

	type WarningMessage struct {
		Warning string `json:"warning"`
	}
	s.Emit("warning", &WarningMessage{
		message,
	})
}

// Sent to the client if either their command was unsuccessful or something else went wrong
func (s *Session) Error(message string) {
	// Specify a default error message
	if message == "" {
		message = DefaultErrorMsg
	}

	logger.Info("Error - " + message + " - " + s.Username)

	type ErrorMessage struct {
		Error string `json:"error"`
	}
	s.Emit("error", &ErrorMessage{
		message,
	})
}

func (s *Session) GetJoinedTable() *Table {
	tablesMutex.RLock()
	defer tablesMutex.RUnlock()

	for _, t := range tables {
		if t.Replay {
			continue
		}

		for _, p := range t.Players {
			if p.ID == s.UserID {
				return t
			}
		}
	}

	return nil
}
