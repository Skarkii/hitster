const lobby = document.getElementById("lobby");
const roomDiv = document.getElementById("room");
const displayNameInput = document.getElementById("displayName");
const roomCodeInput = document.getElementById("roomCode");
const joinForm = document.getElementById("joinForm");
const roomCodeDisplay = document.getElementById("roomCodeDisplay");
const errorDiv = document.getElementById("error");
const roomErrorDiv = document.getElementById("roomError");
const playerList = document.getElementById("playerList");

let ws = null;
let players = {};
let sessionToken = localStorage.getItem("sessionToken") || "";
let currentRoomCode = null;

// Connect to the WebSocket server
function connect() {
  ws = new WebSocket("ws://localhost:8080/ws");

  ws.onopen = () => {
    console.log("Connected to WebSocket server");
    ws.send(
      JSON.stringify({
        type: "announce",
        sessionToken: sessionToken,
      }),
    );
  };

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log(msg);
    switch (msg.type) {
      case "newSession":
        console.log(msg.sessionToken);
        sessionToken = msg.sessionToken;
        localStorage.setItem("sessionToken", sessionToken);
        break;
      case "roomCreated":
      case "roomJoined":
      case "reconnected":
        sessionToken = msg.sessionToken;
        currentRoomCode = msg.roomCode;
        players = msg.players;
        lobby.style.display = "none";
        roomDiv.style.display = "block";
        roomCodeDisplay.textContent = `Room Code: ${currentRoomCode}`;
        updatePlayerList();
        break;
      case "roomState":
        players = msg.players;
        updatePlayerList();
        break;
      case "leftRoom":
        resetToLobby();
        break;
      case "error":
        if (roomDiv.style.display === "block") {
          roomErrorDiv.textContent = msg.error;
        } else {
          errorDiv.textContent = msg.error;
        }
      default:
        console.log("UNHANDLED MSG:" + msg.type);
        break;
    }
  };

  ws.onclose = () => {
    console.log("Disconnected from WebSocket server");
    if (roomDiv.style.display === "block") {
      roomErrorDiv.textContent =
        "Disconnected from server. Attempting to reconnect...";
      setTimeout(connect, 3000); // Attempt to reconnect after 3 seconds
    } else {
      errorDiv.textContent = "Disconnected from server";
    }
  };

  ws.onerror = (error) => {
    console.error("WebSocket error:", error);
    if (roomDiv.style.display === "block") {
      roomErrorDiv.textContent = "Connection error";
    } else {
      errorDiv.textContent = "Connection error";
    }
  };
}

// Update the player list UI
function updatePlayerList() {
  playerList.innerHTML = "";
  for (const i in players) {
    const li = document.createElement("li");
    li.textContent = players[i] + "";
    playerList.appendChild(li);
  }
}

// Reset to lobby screen
function resetToLobby() {
  lobby.style.display = "block";
  roomDiv.style.display = "none";
  roomCodeDisplay.textContent = "";
  roomErrorDiv.textContent = "";
  currentRoomCode = null;
  players = {};
  displayNameInput.value = "";
  roomCodeInput.value = "";
  joinForm.style.display = "none";
}

// Create a new room
function createRoom() {
  console.log("CREATE ROOM!");
  const displayName = displayNameInput.value.trim();
  if (!displayName) {
    errorDiv.textContent = "Please enter a display name";
    return;
  }
  errorDiv.textContent = "";
  ws.send(
    JSON.stringify({
      type: "createRoom",
      displayName: displayName,
    }),
  );
}

// Show the join room form
function showJoinForm() {
  joinForm.style.display = "block";
}

// Join an existing room
function joinRoom() {
  const displayName = displayNameInput.value.trim();
  const code = roomCodeInput.value.trim().toUpperCase();
  if (!displayName) {
    errorDiv.textContent = "Please enter a display name";
    return;
  }
  if (!code) {
    errorDiv.textContent = "Please enter a room code";
    return;
  }
  errorDiv.textContent = "";
  ws.send(
    JSON.stringify({
      type: "joinRoom",
      roomCode: code,
      displayName: displayName,
    }),
  );
}

// Leave the room
function leaveRoom() {
  const code = roomCodeInput.value.trim().toUpperCase();
  ws.send(
    JSON.stringify({
      type: "leaveRoom",
    }),
  );

  // Should I do this?
  //if (ws && ws.readyState === WebSocket.OPEN) {
  //ws.close();
  //}
  //connect();
}

// Start the game
connect();
