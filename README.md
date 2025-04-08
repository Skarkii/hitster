# Hitster Web

**Disclaimer:** This project is a work in progress and not yet ready for full use. Features are still being developed, and bugs may be present.

## About Hitster

Hitster is a fun, music-based party game where players guess the release year of popular songs and place them in a timeline. Originally a card game, this web-based version aims to bring the same excitement to your browser, allowing friends to play together online with a digital twist. Test your music knowledge and compete to create the perfect timeline!

## Project Overview

This repository contains a web-based implementation of Hitster, built with:
- **Backend:** Go (Golang) for handling game logic, API endpoints, and server-side operations.
- **Frontend:** Plain HTML and JavaScript using Websockets for a lightweight, interactive user interface.

The goal is to replicate the core gameplay of Hitster while adding online multiplayer capabilities.

## Features

### Implemented
- Lobby creation/joining/leaving with generated codes.
- Multiplayer support.
- Session tokens to restore a users existing sessions


### To Be Added (Priority top-down)
- Basic game with timers, round based, in total, the entire game.
- Results screen after a finished game
- Improved UI with animations and responsive design.
- Support fetching song lists from github repos.

## Getting Started

This section is to be improved upon a more ready state.

1. **Clone the repository:**
   ```bash
   git clone https://github.com/skarkii/hitster.git
   ```
 2. **Start the backend:**
  ```bash
  cd backend
  go run .
  ```
3. **Open the index.html**
   
  Open your browser and navigate to the file frontend/index.html

4. **(Optional) Open a webserver for index.html**
   Instructions to be added.
