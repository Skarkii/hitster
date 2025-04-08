package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Client -> Server messages
type ClientMessage struct {
	Type         string `json:"type"`
	SessionToken string `json:"sessionToken"`
	RoomCode     string `json:"roomCode"`
	DisplayName  string `json:"displayName"`
}

// Server -> Client messages
type ServerMessage struct {
	Type         string   `json:"type"`
	SessionToken string   `json:"sessionToken"`
	RoomCode     string   `json:"roomCode"`
	RoomOwner    bool     `json:"roomOwner"`
	Players      []string `json:"players"`
	State        string   `json:"state"`
}

// Player struct
type Player struct {
	SessionToken string
	DisplayName  string
	RoomID       string
	Conn         *websocket.Conn
}

// Room struct
type Room struct {
	ID      string
	Host    *Player
	Players map[string]*Player
	Lock    sync.Mutex
	State   string
}

// GameState holds all players(sessions) and rooms
type GameState struct {
	Sessions map[string]*Player
	Rooms    map[string]*Room
	Lock     sync.Mutex
}

var game = GameState{
	Sessions: make(map[string]*Player),
	Rooms:    make(map[string]*Room),
}

func printAllPlayers() {
	game.Lock.Lock()
	defer game.Lock.Unlock()
	fmt.Println("All players:")
	for key := range game.Sessions {
		fmt.Println(key)
	}
	fmt.Println("END All players")
}

func printAllPlayersInRoom(room *Room) {
	room.Lock.Lock()
	defer room.Lock.Unlock()
	fmt.Printf("All players in room %s:\n", room.ID)
	for key := range room.Players {
		fmt.Println(room.Players[key])
	}
	fmt.Printf("END All players in room %s\n", room.ID)
}

// Generate a random room code
func generateRoomCode() string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	code := make([]byte, 4)
	for i := range code {
		code[i] = letters[rand.Intn(len(letters))]
	}
	return string(code)
}

// Generates a new player
func generatePlayer() *Player {
	// Generate a unique token between 100000000000-999999999999
	game.Lock.Lock()
	sessionToken := fmt.Sprintf("%d", 100000000000+rand.Intn(899999999999))
	for _, exists := game.Sessions[sessionToken]; exists; {
		sessionToken = fmt.Sprintf("%d", 100000000000+rand.Intn(899999999999))
	}

	// Create a player with an unique session Token
	player := &Player{
		SessionToken: sessionToken,
	}

	game.Sessions[player.SessionToken] = player
	game.Lock.Unlock()

	return player
}

// Returns the player from the token if it exists
func getPlayer(token string) *Player {
	game.Lock.Lock()
	defer game.Lock.Unlock()
	if existingPlayer, exists := game.Sessions[token]; exists {
		return existingPlayer
	}
	return nil
}

func createRoom(host string, displayName string) *Room {
	hostUser := getPlayer(host)
	if hostUser == nil {
		fmt.Printf("Nil user %s tried to create a room!", host)
		return nil
	}
	hostUser.DisplayName = displayName
	game.Lock.Lock()
	defer game.Lock.Unlock()
	// Generate a room code until given a new room
	roomCode := generateRoomCode()
	for _, exists := game.Rooms[roomCode]; exists; {
		roomCode = generateRoomCode()
	}

	room := &Room{
		ID:      roomCode,
		Host:    hostUser,
		Players: make(map[string]*Player),
		State:   "lobby",
	}

	game.Rooms[room.ID] = room
	fmt.Printf("Player %s created room %s!\n", hostUser.SessionToken, roomCode)

	room.Players[hostUser.SessionToken] = hostUser

	return room
}

func getRoom(code string, playerToken string, displayName string) *Room {
	player := getPlayer(playerToken)
	if player == nil {
		fmt.Printf("Nil user %s tried to get room!!", playerToken)
		return nil
	}
	player.DisplayName = displayName
	game.Lock.Lock()
	defer game.Lock.Unlock()

	if existingRoom, exists := game.Rooms[code]; exists {
		existingRoom.Lock.Lock()

		existingRoom.Players[player.SessionToken] = player

		existingRoom.Lock.Unlock()

		return existingRoom
	}
	return nil
}

func getPlayerNames(room *Room) []string {
	room.Lock.Lock()
	defer room.Lock.Unlock()
	var players []string
	for key := range room.Players {
		players = append(players, room.Players[key].DisplayName)
	}
	return players
}

// Broadcast the current room state to all players in the room
func broadcastRoomState(room *Room) {
	names := getPlayerNames(room)
	room.Lock.Lock()
	players := room.Players
	room.Lock.Unlock()

	for _, player := range players {
		response := ServerMessage{
			Type:      "roomState",
			RoomCode:  room.ID,
			RoomOwner: isRoomOwner(room, player),
			Players:   names,
			State:     room.State,
		}
		if player.Conn != nil {
			player.Conn.WriteJSON(response)
		}
	}
}

func isRoomOwner(room *Room, player *Player) bool {
	if room == nil {
		return false
	}
	room.Lock.Lock()
	game.Lock.Lock()

	defer room.Lock.Unlock()
	defer game.Lock.Unlock()

	if player == room.Host {
		return true
	}
	return false
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	defer conn.Close()
	/*defer func() {
	conn.Close()
	}()*/

	fmt.Printf("New connection from: %s\n", conn.LocalAddr())

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			// Ignore the error if it's a normal closure or going away
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				return
			}
			log.Printf("Error reading message: %v", err)
			break
		}

		var clientMsg ClientMessage
		if err := json.Unmarshal(msg, &clientMsg); err != nil {
			log.Printf("Error parsing message: %v", err)
			continue
		}

		switch clientMsg.Type {
		case "announce":
			var player *Player
			player = getPlayer(clientMsg.SessionToken)

			if player == nil {
				player = generatePlayer()
			}

			player.Conn = conn

			conn.WriteJSON(ServerMessage{
				Type:         "session",
				SessionToken: player.SessionToken,
			})

			room := getRoom(player.RoomID, player.SessionToken, player.DisplayName)

			if room != nil {
				conn.WriteJSON(ServerMessage{
					Type:      "joinedRoom",
					RoomCode:  room.ID,
					RoomOwner: isRoomOwner(room, player),
					Players:   getPlayerNames(room),
				})
				broadcastRoomState(room)
			}

		case "startGame":
			player := getPlayer(clientMsg.SessionToken)
			room := getRoom(player.RoomID, player.SessionToken, player.DisplayName)

			if room == nil {
				fmt.Printf("Player tried to start but not in room")
				conn.WriteJSON(ServerMessage{
					Type: "notInRoom",
				})
				continue
			}

			room.Lock.Lock()
			if len(room.Players) <= 1 {
				fmt.Printf("Player tried to start room without enough players!")
				room.Lock.Unlock()
				conn.WriteJSON(ServerMessage{
					Type: "notEnoughPlayers",
				})
				continue
			}
			room.State = "playing"
			room.Lock.Unlock()

			broadcastRoomState(room)

		case "createRoom":
			room := createRoom(clientMsg.SessionToken, clientMsg.DisplayName)
			player := getPlayer(clientMsg.SessionToken)

			conn.WriteJSON(ServerMessage{
				Type:      "joinedRoom",
				RoomCode:  room.ID,
				RoomOwner: true,
				Players:   getPlayerNames(room),
			})
			if player == nil {
				fmt.Printf("nil player %s tried to create room.\n", clientMsg.SessionToken)
			}
			player.RoomID = room.ID

		case "joinRoom":
			room := getRoom(clientMsg.RoomCode, clientMsg.SessionToken, clientMsg.DisplayName)
			if room == nil {
				conn.WriteJSON(ServerMessage{
					Type: "failedJoin",
				})
				continue
			}
			conn.WriteJSON(ServerMessage{
				Type:      "joinedRoom",
				RoomCode:  room.ID,
				RoomOwner: false,
				Players:   getPlayerNames(room),
			})
			player := getPlayer(clientMsg.SessionToken)
			if player == nil {
				fmt.Printf("nil player %s tried to join room.\n", clientMsg.SessionToken)
				return
			}
			player.RoomID = room.ID

			broadcastRoomState(room)

		case "leaveRoom":
			player := getPlayer(clientMsg.SessionToken)
			if player == nil {
				fmt.Printf("Nil player %s tried to leave room!", clientMsg.SessionToken)
				return
			}
			room := getRoom(player.RoomID, player.SessionToken, player.DisplayName)
			if room == nil {
				fmt.Printf("Player %s tried to leave non existing room!\n", clientMsg.SessionToken)
				continue
			}

			player.RoomID = ""

			room.Lock.Lock()
			delete(room.Players, player.SessionToken)
			if len(room.Players) <= 0 {
				fmt.Printf("Room empty, removing")
				room.Lock.Unlock()
				game.Lock.Lock()
				delete(game.Rooms, room.ID)
				game.Lock.Unlock()
				continue
			}
			room.Lock.Unlock()

			if isRoomOwner(room, player) {
				room.Lock.Lock()
				for _, player := range room.Players {
					room.Host = player
					break
				}
				room.Lock.Unlock()
			}

			broadcastRoomState(room)

		default:
			fmt.Printf("Unhandled!: %s\n", clientMsg.Type)
		}
	}
}

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins (restrict in production)
	},
}

func main() {
	http.HandleFunc("/ws", handleWebSocket)

	fmt.Println("Server listening on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
