package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Player represents a player in the game
type Player struct {
	SessionID    string          `json:"-"`            // Unique session ID (not sent to client)
	SessionToken string          `json:"sessionToken"` // Token for reconnection
	DisplayName  string          `json:"displayName"`  // Player's chosen name
	Conn         *websocket.Conn `json:"-"`            // WebSocket connection
	RoomID       string          `json:"-"`            // Room the player is in
	LastSeen     time.Time       `json:"-"`            // Last time the player was active
}

// Room represents a game room
type Room struct {
	ID      string             `json:"id"`
	Host    *Player            // The first person to join
	Players map[string]*Player // Keyed by sessionID
	Lock    sync.Mutex
}

// GameState holds all rooms and sessions
type GameState struct {
	Rooms    map[string]*Room
	Sessions map[string]*Player // Keyed by sessionToken
	Lock     sync.Mutex
}

var game = GameState{
	Rooms:    make(map[string]*Room),
	Sessions: make(map[string]*Player),
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (restrict in production)
	},
}

// Generate a random room code
func generateRoomCode() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	rand.Seed(time.Now().UnixNano())
	code := make([]byte, 4)
	for i := range code {
		code[i] = letters[rand.Intn(len(letters))]
	}
	return string(code)
}

// Clean up old sessions periodically
func cleanupOldSessions() {
	for {
		time.Sleep(1 * time.Minute)
		log.Printf("Performing cleanup")
		game.Lock.Lock()
		for token, player := range game.Sessions {
			if time.Since(player.LastSeen) > 5*time.Minute { // 5-minute timeout
				if player.RoomID != "" {
					if room, exists := game.Rooms[player.RoomID]; exists {
						room.Lock.Lock()
						delete(room.Players, player.SessionID)
						room.Lock.Unlock()
						broadcastRoomState(room)
					}
				}
				delete(game.Sessions, token)
			}
		}
		game.Lock.Unlock()
	}
}

// Message types for client-server communication
type ClientMessage struct {
	Type         string `json:"type"`
	RoomCode     string `json:"roomCode"`
	DisplayName  string `json:"displayName"`
	SessionToken string `json:"sessionToken"`
}

type ServerMessage struct {
	Type     string   `json:"type"`
	RoomCode string   `json:"roomCode"`
	Players  []string `json:"players"`
	Error    string   `json:"error"`
}

func getPlayerNames(room *Room) []string {
	room.Lock.Lock()
	defer room.Lock.Unlock()
	var names []string
	for _, player := range room.Players {
		names = append(names, player.DisplayName)
	}
	return names
}

func generate_player(conn *websocket.Conn) *Player {
	sessionID := fmt.Sprintf("session-%d", rand.Intn(1000000))
	sessionToken := fmt.Sprintf("token-%d", rand.Intn(1000000))
	// Create a player with an unique session ID & Token
	player := &Player{
		SessionID:    sessionID,
		SessionToken: sessionToken,
		Conn:         conn,
		LastSeen:     time.Now(),
	}

	game.Lock.Lock()
	game.Sessions[player.SessionToken] = player
	fmt.Printf("Created session {%s}", sessionToken)
	game.Lock.Unlock()

	// Send room code and session token to the client
	response := ClientMessage{
		Type:         "newSession",
		SessionToken: sessionToken,
	}
	conn.WriteJSON(response)
	return player
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	var player *Player
	var room *Room

	defer func() {
		if player != nil && room != nil {
			player.Conn = nil
			player.LastSeen = time.Now()
			broadcastRoomState(room)
		}
		conn.Close()
	}()

	// Listen for messages from the client
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		switch clientMsg.Type {
		// Upon socket creation an announce is sent to either reconnect or generate new session
		case "announce":
			// Handle sessions by reconnecting or creating a new Session ID
			fmt.Printf("SENT %s\n", clientMsg.SessionToken)
			if clientMsg.SessionToken != "" {
				game.Lock.Lock()
				if existingPlayer, exists := game.Sessions[clientMsg.SessionToken]; exists {
					fmt.Printf("Existing session found, reconnecting!")
					existingPlayer.Conn = conn
					existingPlayer.LastSeen = time.Now()
					player = existingPlayer
					if room, exists = game.Rooms[player.RoomID]; exists {
						room.Lock.Lock()
						room.Players[player.SessionID] = player
						room.Lock.Unlock()
						conn.WriteJSON(ClientMessage{
							Type:         "reconnected",
							RoomCode:     room.ID,
							SessionToken: player.SessionToken,
						})
						broadcastRoomState(room)
						game.Lock.Unlock()
						continue
					}
				} else {
					game.Lock.Unlock()
					player = generate_player(conn)
				}
			} else {
				player = generate_player(conn)
			}
		case "createRoom":
			// Generate a room with a unique non used ID
			game.Lock.Lock()
			roomCode := generateRoomCode()
			for _, exists := game.Rooms[roomCode]; exists; {
				roomCode = generateRoomCode()
			}
			room = &Room{
				ID:      roomCode,
				Players: make(map[string]*Player),
				Host:    player,
			}

			game.Rooms[roomCode] = room
			game.Lock.Unlock()

			player.RoomID = room.ID
			player.DisplayName = clientMsg.DisplayName

			room.Lock.Lock()
			room.Players[player.SessionID] = player
			room.Lock.Unlock()

			// Send current room state and session token to the new player
			response := ClientMessage{
				Type:        "roomCreated",
				RoomCode:    room.ID,
				DisplayName: clientMsg.DisplayName,
			}
			conn.WriteJSON(response)
			broadcastRoomState(room)

		case "joinRoom":
			if player.RoomID != "" {
				conn.WriteJSON(ServerMessage{
					Type:  "error",
					Error: "Already in a room",
				})
				continue
			}

			game.Lock.Lock()
			room, exists := game.Rooms[clientMsg.RoomCode]
			game.Lock.Unlock()

			if !exists {
				conn.WriteJSON(ServerMessage{
					Type:  "error",
					Error: "Room not found",
				})
				continue
			}

			player.DisplayName = clientMsg.DisplayName
			player.RoomID = clientMsg.RoomCode

			room.Lock.Lock()
			room.Players[player.SessionID] = player
			room.Lock.Unlock()

			// Send current room state and session token to the new player
			response := ClientMessage{
				Type:        "roomJoined",
				RoomCode:    room.ID,
				DisplayName: clientMsg.DisplayName,
			}

			conn.WriteJSON(response)
			broadcastRoomState(room)

		case "leaveRoom":
			if player.RoomID == "" {
				// Player isn't in a room, just confirm they've left
				conn.WriteJSON(ServerMessage{
					Type: "leftRoom",
				})
				fmt.Printf("Finished here")
				continue
			}
			// Fetch the room using player.RoomID
			game.Lock.Lock()
			room, exists := game.Rooms[player.RoomID]
			if exists {
				room.Lock.Lock()
				delete(room.Players, player.SessionID)
				room.Lock.Unlock()
			}
			game.Lock.Unlock()

			player.RoomID = ""
			broadcastRoomState(room)
			// Notify the player they've left
			conn.WriteJSON(ServerMessage{
				Type: "leftRoom",
			})

		default:
			fmt.Printf("Unhandled!: %s\n", clientMsg.Type)
		}
	}
}

// Broadcast the current room state to all players in the room
func broadcastRoomState(room *Room) {
	names := getPlayerNames(room)
	room.Lock.Lock()
	defer room.Lock.Unlock()

	if len(room.Players) > 0 {
		fmt.Printf("Players: %v\n", room.Players)
	} else {
		fmt.Printf("No players!\n")
	}

	response := ServerMessage{
		Type:     "roomState",
		RoomCode: room.ID,
		Players:  names,
	}
	for _, player := range room.Players {
		if player.Conn != nil {
			player.Conn.WriteJSON(response)
		}
	}
}

func main() {
	// Start cleanup routine
	go cleanupOldSessions()

	// Serve WebSocket endpoint
	http.HandleFunc("/ws", handleWebSocket)

	fmt.Println("Server listening on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
