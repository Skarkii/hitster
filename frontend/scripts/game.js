let sessionToken = localStorage.getItem("sessionToken") || "";
const roomCodeDisplay = document.getElementById("roomCodeDisplay");
const roomCodeInput = document.getElementById("roomCode");
const displayNameInput = document.getElementById("displayName");
const lobbyDiv = document.getElementById("lobby");
const homepageDiv = document.getElementById("homepage");
const gameDiv = document.getElementById("game");
const startButton = document.getElementById("startButton");
const currentSong = document.getElementById("currentSong");
let ws = null;
let isOwner = false;
let currentRoomCode = null;
let players = {};

const popupTypes = Object.freeze({
  INFO: 0,
  WARNING: 1,
  ERROR: 2,
});

let currentPopupType = popupTypes.INFO;

function showPopup(message, type = popupTypes.INFO) {
  // Ignore if worse message is already pending
  if (type < currentPopupType) {
    console.log("IGNORED POPUP CURRENT: ", currentPopupType);
    return;
  }

  console.log("SHOWING POPUP: ", type, "Current: ", currentPopupType);
  currentPopupType = type;
  const popup = document.getElementById("popup");
  popup.textContent = message;
  popup.className = "popup"; // Reset classes
  if (type === popupTypes.WARNING) popup.classList.add("warning");
  if (type === popupTypes.ERROR) popup.classList.add("error");
  popup.style.display = "block";

  // If it is info, hide after set time
  if (type === popupTypes.INFO) {
    setTimeout(() => {
      hidePopup();
    }, 3000);
  }
}

function hidePopup(fromOpen = false) {
  if (fromOpen === true) {
    if (currentPopupType <= popupTypes.INFO) {
      console.log("Ignored hide because of startup");
      return;
    }
  }
  console.log("HIDES NOW!");
  const popup = document.getElementById("popup");
  popup.classList.remove("error");
  popup.classList.remove("warning");
  popup.style.display = "none";
  currentPopupType = popupTypes.INFO;
}

function createRoom() {
  const displayName = displayNameInput.value.trim();
  if (!displayName) {
    showPopup("Please enter a display name!");
    return;
  }
  ws.send(
    JSON.stringify({
      type: "createRoom",
      displayName: displayName,
      sessionToken: sessionToken,
    }),
  );
}

function joinRoom() {
  const displayName = displayNameInput.value.trim();
  const code = roomCodeInput.value.trim().toUpperCase();
  if (!displayName) {
    showPopup("Please enter a display name!");
    return;
  }
  ws.send(
    JSON.stringify({
      type: "joinRoom",
      displayName: displayName,
      sessionToken: sessionToken,
      roomCode: code,
    }),
  );
}

// Update the player list UI
function updatePlayerList() {
  homepageDiv.style.display = "None";
  lobbyDiv.style.display = "Block";
  gameDiv.style.display = "None";

  playerList.innerHTML = "";
  for (const i in players) {
    const li = document.createElement("li");
    li.textContent = players[i] + "";
    playerList.appendChild(li);
  }
  if (isOwner) {
    startButton.style.display = "Block";
  } else {
    startButton.style.display = "none";
  }
}

function startGame() {
  ws.send(
    JSON.stringify({
      type: "startGame",
      sessionToken: sessionToken,
    }),
  );
}

function joinedRoom() {
  roomCodeDisplay.textContent = `Room Code: ${currentRoomCode}`;
  updatePlayerList();

  homepageDiv.style.display = "None";
  lobbyDiv.style.display = "Block";
  gameDiv.style.display = "None";
}

function leaveRoom() {
  homepageDiv.style.display = "Block";
  lobbyDiv.style.display = "None";
  gameDiv.style.display = "None";
  currentRoomCode = null;
  ws.send(
    JSON.stringify({
      type: "leaveRoom",
      sessionToken: sessionToken,
    }),
  );
}

function displayGame() {
  gameDiv.style.display = "Block";
  lobbyDiv.style.display = "none";
  homepageDiv.style.display = "none";
}

function connect() {
  ws = new WebSocket("ws://localhost:8080/ws");
  console.log("Connecting to websocket!");
  const timeout = setTimeout(() => {
    console.error("WebSocket connection timeout!");
    ws.close();
  }, 2500); // 5 seconds

  ws.onopen = () => {
    clearTimeout(timeout);
    console.log("Connected to server");
    ws.send(
      JSON.stringify({
        type: "announce",
        sessionToken: sessionToken,
      }),
    );
    hidePopup(true);
  };

  ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    console.log(msg);
    switch (msg.type) {
      case "session":
        sessionToken = msg.sessionToken;
        localStorage.setItem("sessionToken", sessionToken);
        console.log("Set SessionToken to: %s", sessionToken);
        break;
      case "notEnoughPlayers":
        showPopup("Not enough players!", popupTypes.INFO);
        break;
      case "notInRoom":
        showPopup(
          "Not in room, please refresh your browser!",
          popupTypes.WARNING,
        );
        break;
      case "failedJoin":
        showPopup("Room code does not exist!");
        break;
      case "joinedRoom":
        console.log("Joined room!");
        currentRoomCode = msg.roomCode;
        players = msg.players;
        isOwner = msg.roomOwner;
        joinedRoom();
        break;
      case "roomState":
        players = msg.players;
        isOwner = msg.roomOwner;
        currentSong.innerHTML = "Current Song: " + msg.song;
        if (msg.state === "lobby") {
          updatePlayerList();
        } else if (msg.state === "playing") {
          displayGame();
        } else {
          console.log("UNPARSED GAME STATE: " + msg.state);
        }
        break;
      default:
        console.log("UNHANDLED MSG:" + msg.type);
        break;
    }
  };

  ws.onclose = (event) => {
    // Ignore if the closure was because of a refresh
    if (event.code === 1001 && event.wasClean) {
      console.log("Close was clean, therefore ignored");
      return;
    }
    console.log("WebSocket disconnected.. reconnecting!!: ", event);
    showPopup("WebSocket disconnected.. reconnecting!!", popupTypes.WARNING);
    setTimeout(connect, 1500);
  };

  ws.onerror = (error) => {
    // I do not think this should be printed.
    //showPopup("WebSocket error:", popupTypes.ERROR);
    console.log("WEBSOCKET ERROR");
    console.error("WebSocket error:", error);
    ws.close();
  };
}

connect();
