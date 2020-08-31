package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	gsessions "github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v4"
)

var (
	upgrader = websocket.Upgrader{}
)

// httpWS handles part 2 of 2 for logic authentication
// Part 1 is found in "httpLogin.go"
// After receiving a cookie in part 1, the client will attempt to open a WebSocket connection with
// the cookie (this is done implicitly because JavaScript will automatically use any current cookies
// for the website when establishing a WebSocket connection)
// So, before allowing anyone to open a WebSocket connection, we need to validate that they have
// gone through part 1 (e.g. they have a valid cookie that was created at some point in the past)
// We also do a few other checks to be thorough
// If all of the checks pass, the WebSocket connection will be established,
// and then the user's website data will be initialized in "websocketConnect.go"
// If anything fails in this function, we want to delete the user's cookie in order to force them to
// start authentication from the beginning
func httpWS(c *gin.Context) {
	// Local variables
	r := c.Request
	w := c.Writer

	// Parse the IP address
	var ip string
	if v, _, err := net.SplitHostPort(r.RemoteAddr); err != nil {
		msg := "Failed to parse the IP address:"
		httpWSInternalError(c, msg, err)
		return
	} else {
		ip = v
	}

	logger.Debug("Entered the \"httpWS()\" function for IP: " + ip)

	// Check to see if their IP is banned
	if banned, err := models.BannedIPs.Check(ip); err != nil {
		msg := "Failed to check to see if the IP \"" + ip + "\" is banned:"
		httpWSInternalError(c, msg, err)
		return
	} else if banned {
		msg := "IP \"" + ip + "\" tried to establish a WebSocket connection, but they are banned."
		reason := "Your IP address has been banned. Please contact an administrator if you think this is a mistake."
		httpWSDeny(c, msg, reason)
		return
	}

	// Check to see if their IP is muted
	var muted bool
	if v, err := models.MutedIPs.Check(ip); err != nil {
		msg := "Failed to check to see if the IP \"" + ip + "\" is muted:"
		httpWSInternalError(c, msg, err)
		return
	} else {
		muted = v
	}

	// If they have a valid cookie, it should have the "userID" value that we set in "httpLogin()"
	session := gsessions.Default(c)
	var userID int
	if v := session.Get("userID"); v == nil {
		msg := "Unauthorized WebSocket handshake detected from \"" + ip + "\". " +
			"This likely means that their cookie has expired."
		httpWSDeny(c, msg, "")
		return
	} else {
		userID = v.(int)
	}

	// Get the username for this user
	var username string
	if v, err := models.Users.GetUsername(userID); err == pgx.ErrNoRows {
		// The user has a cookie for a user that does not exist in the database,
		// e.g. an "orphaned" user
		// This can happen in situations where a test user was deleted, for example
		// Delete their cookie and force them to re-login
		msg := "User from \"" + ip + "\" " +
			"tried to login with a cookie with an orphaned user ID of " + strconv.Itoa(userID) +
			". Deleting their cookie."
		httpWSDeny(c, msg, "")
		return
	} else if err != nil {
		msg := "Failed to get the username for user " + strconv.Itoa(userID) + ":"
		httpWSInternalError(c, msg, err)
		return
	} else {
		username = v
	}

	// Get their friends and reverse friends
	var friendsMap map[int]struct{}
	if v, err := models.UserFriends.GetMap(userID); err != nil {
		msg := "Failed to get the friend map for user \"" + username + "\":"
		httpWSInternalError(c, msg, err)
		return
	} else {
		friendsMap = v
	}
	var reverseFriendsMap map[int]struct{}
	if v, err := models.UserReverseFriends.GetMap(userID); err != nil {
		msg := "Failed to get the reverse friend map for user \"" + username + "\":"
		httpWSInternalError(c, msg, err)
		return
	} else {
		reverseFriendsMap = v
	}

	// Get whether or not they are a member of the Hyphen-ated group
	var hyphenated bool
	if v, err := models.UserSettings.IsHyphenated(userID); err != nil {
		msg := "Failed to get the Hyphen-ated setting for user \"" + username + "\":"
		httpWSInternalError(c, msg, err)
		return
	} else {
		hyphenated = v
	}

	// If they got this far, they are a valid user
	logger.Info("User \"" + username + "\" is establishing a WebSocket connection.")
	var conn *websocket.Conn
	if v, err := upgrader.Upgrade(w, r, nil); err != nil {
		// WebSocket establishment can fail for mundane reasons (e.g. internet dropping)
		logger.Info("Failed to upgrade the HTTP connection for user \""+username+"\":", err)
		http.Error(
			w,
			http.StatusText(http.StatusBadRequest),
			http.StatusBadRequest,
		)
		deleteCookie(c)
		return
	} else {
		conn = v
	}
	defer conn.Close()

	// Initialize the object that represents their WebSocket session
	s := NewSession()
	s.Conn = conn
	s.UserID = userID
	s.Username = username
	s.Muted = muted
	s.Friends = friendsMap
	s.ReverseFriends = reverseFriendsMap
	s.Hyphenated = hyphenated

	for {
		// Read
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logger.Error(err)
		}
		fmt.Printf("%s\n", msg)
	}
}

func httpWSInternalError(c *gin.Context, msg string, err error) {
	// Local variables
	w := c.Writer

	logger.Error(msg, err)
	http.Error(
		w,
		http.StatusText(http.StatusInternalServerError),
		http.StatusInternalServerError,
	)
	deleteCookie(c)
}

func httpWSDeny(c *gin.Context, msg string, reason string) {
	// Local variables
	w := c.Writer

	logger.Info(msg)
	if reason == "" {
		reason = http.StatusText(http.StatusUnauthorized)
	}
	http.Error(
		w,
		reason,
		http.StatusUnauthorized,
	)
	deleteCookie(c)
}
