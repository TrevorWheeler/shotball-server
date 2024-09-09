package lobby

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"myapp/src/authentication"
	"myapp/src/types"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

type SafeConnection struct {
	Conn  *websocket.Conn
	Mutex sync.Mutex
}

var (
	activeConnections = make(map[string]*SafeConnection)
	connMutex         sync.Mutex
)

func addConnection(userID string, conn *websocket.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()
	activeConnections[userID] = &SafeConnection{Conn: conn}
}

var globalGameState = struct {
	sync.RWMutex
	Lobbies map[string]*GameState
}{
	Lobbies: make(map[string]*GameState),
}

type GameState struct {
	GameID       string       `json:"gameId"`
	Players      []Player     `json:"players"`
	Projectiles  []Projectile `json:"projectiles"`
	LastActivity time.Time
}

type Player struct {
	PlayerID        string                `json:"playerId"`
	Username        string                `json:"username"`
	Health          float64               `json:"health"`
	PositionX       float64               `json:"positionX"`
	PositionY       float64               `json:"positionY"`
	TargetVelocityY float64               `json:"targetVelocityY"`
	TargetVelocityX float64               `json:"targetVelocityX"`
	VelocityY       float64               `json:"velocityY"`
	VelocityX       float64               `json:"velocityX"`
	Angle           float64               `json:"angle"`
	MousePositionY  float64               `json:"mousePositionY"`
	MousePositionX  float64               `json:"mousePositionX"`
	Controls        types.PlayerDirection `json:"controls"`
}

type FrontendGameState struct {
	GameID      string       `json:"gameId"`
	Players     []Player     `json:"players"`
	Projectiles []Projectile `json:"projectiles"`
}

type FrontendGameEnter struct {
	Token     string            `json:"token"`
	GameState FrontendGameState `json:"gameState"`
}

type Projectile struct {
	ProjectileID string  `json:"projectileId"`
	PlayerID     string  `json:"playerId"`
	PositionX    float64 `json:"positionX"`
	PositionY    float64 `json:"positionY"`
	VelocityX    float64 `json:"velocityX"`
	VelocityY    float64 `json:"velocityY"`
}

func CreateGame(c echo.Context, ws *websocket.Conn, requestData map[string]interface{}) error {
	newLobby := &GameState{
		GameID:      uuid.New().String(),
		Players:     []Player{},
		Projectiles: []Projectile{},
	}

	globalGameState.Lock()
	globalGameState.Lobbies[newLobby.GameID] = newLobby
	globalGameState.Unlock()

	response := types.FrontendResponse{
		ID:   "game_created",
		Data: newLobby.GameID,
	}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return nil
	}

	err = ws.WriteMessage(websocket.TextMessage, jsonResponse)
	if err != nil {
		return err
	}
	return nil
}

type LobbyRequest struct {
	LobbyId  string `json:"lobbyId"`
	Username string `json:"username"`
}

type LobbyResponse struct {
	LobbyID string   `json:"lobbyId"`
	Players []Player `json:"players"`
}

func JoinGame(c echo.Context, ws *websocket.Conn, requestData map[string]interface{}) error {

	requestBytes, err := json.Marshal(requestData)
	if err != nil {
		return fmt.Errorf("error marshaling request data: %v", err)
	}

	var lobbyRequest LobbyRequest
	if err := json.Unmarshal(requestBytes, &lobbyRequest); err != nil {
		return fmt.Errorf("error unmarshaling into LobbyRequest: %v", err)
	}

	if lobbyRequest.LobbyId == "" {
		return fmt.Errorf("lobby id not provided")
	}

	if lobbyRequest.Username == "" {
		return fmt.Errorf("username not provided")
	}

	playerId := uuid.New().String()

	signedToken, err := authentication.GenerateToken(lobbyRequest.Username, lobbyRequest.LobbyId, playerId)
	if err != nil {
		return fmt.Errorf("failed to generate user token")
	}

	player := Player{
		PlayerID:        playerId,
		Username:        lobbyRequest.Username,
		Health:          100,
		PositionX:       500,
		PositionY:       500,
		TargetVelocityX: 0,
		TargetVelocityY: 0,
		VelocityX:       0,
		VelocityY:       0,
		Angle:           0,
		MousePositionX:  0,
		MousePositionY:  0,
		Controls: types.PlayerDirection{
			Up:    false,
			Down:  false,
			Left:  false,
			Right: false,
		},
	}

	globalGameState.Lock()

	if lobby, ok := globalGameState.Lobbies[lobbyRequest.LobbyId]; ok {
		lobby.Players = append(lobby.Players, player)
		response := types.FrontendResponse{
			ID: "game_enter",
			Data: FrontendGameEnter{
				Token: signedToken,
				GameState: FrontendGameState{
					GameID:      lobbyRequest.LobbyId,
					Players:     lobby.Players,
					Projectiles: lobby.Projectiles,
				},
			},
		}
		jsonResponse, err := json.Marshal(response)
		if err != nil {
			return nil
		}
		addConnection(playerId, ws)
		err = ws.WriteMessage(websocket.TextMessage, jsonResponse)
		if err != nil {
			return err
		}
	} else {
		c.Logger().Error("Problem")
	}
	globalGameState.Unlock()
	updateLobbyActivity(lobbyRequest.LobbyId)
	return nil
}

func PlayerUpdatePosition(c echo.Context, ws *websocket.Conn, requestData map[string]interface{}, tokenString string) error {
	token, claims, err := authentication.ParseToken(tokenString)
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("invalid token")
	}
	up, ok := requestData["up"].(bool)
	if !ok {
		return fmt.Errorf("up not provided")
	}

	down, ok := requestData["down"].(bool)
	if !ok {
		return fmt.Errorf("up not provided")
	}

	left, ok := requestData["left"].(bool)
	if !ok {
		return fmt.Errorf("left not provided")
	}
	right, ok := requestData["right"].(bool)
	if !ok {
		return fmt.Errorf("left not provided")
	}

	mousePositionX, ok := requestData["mousePositionX"].(float64)
	if !ok {
		c.Logger().Panic()
		return fmt.Errorf("mouse x not provided")
	}

	mousePositionY, ok := requestData["mousePositionY"].(float64)
	if !ok {
		return fmt.Errorf("mouse y not provided")
	}
	playerId := claims.PlayerID
	gameId := claims.GameId

	globalGameState.Lock()
	if lobby, ok := globalGameState.Lobbies[gameId]; ok {
		// Iterate through players to find the matching one
		for i := range lobby.Players {
			if lobby.Players[i].PlayerID == playerId {
				lobby.Players[i].Controls.Up = up
				lobby.Players[i].Controls.Down = down
				lobby.Players[i].Controls.Left = left
				lobby.Players[i].Controls.Right = right
				lobby.Players[i].MousePositionX = mousePositionX
				lobby.Players[i].MousePositionY = mousePositionY
				break
			}
		}
	}
	globalGameState.Unlock()

	return nil
}

type Point struct {
	X float64
	Y float64
}

func PlayerShootProjectile(c echo.Context, ws *websocket.Conn, requestData map[string]interface{}, tokenString string) error {
	token, claims, err := authentication.ParseToken(tokenString)
	if err != nil {
		return err
	}
	if !token.Valid {
		return fmt.Errorf("invalid token")
	}

	playerId := claims.PlayerID
	gameId := claims.GameId

	globalGameState.Lock()
	defer globalGameState.Unlock()

	if lobby, ok := globalGameState.Lobbies[gameId]; ok {
		for i := range lobby.Players {
			if lobby.Players[i].PlayerID == playerId {
				player := &lobby.Players[i]
				angle := player.Angle
				x := player.PositionX
				y := player.PositionY

				// Define the tip of the triangle relative to the center of the spacecraft
				triangleHeight := 30.0 // Distance from the center to the tip of the triangle
				tipPosition := rotateAndTranslate(Point{X: 0, Y: -triangleHeight}, angle, x, y)

				// Calculate projectile velocity towards the mouse position
				projectileVelocity := calculateProjectileVelocity(
					tipPosition.X,
					tipPosition.Y,
					player.MousePositionX,
					player.MousePositionY,
					13.0, // Speed of the projectile
				)

				// Create the projectile starting at the tip of the triangle
				projectile := Projectile{
					ProjectileID: uuid.New().String(),
					PlayerID:     playerId,
					PositionX:    tipPosition.X,
					PositionY:    tipPosition.Y,
					VelocityX:    projectileVelocity.X,
					VelocityY:    projectileVelocity.Y,
				}

				// Add projectile to the lobby
				lobby.Projectiles = append(lobby.Projectiles, projectile)

				break
			}
		}
	} else {
		c.Logger().Error("Problem")
	}

	return nil
}

func calculateProjectileVelocity(originX, originY, targetX, targetY, speed float64) Point {
	dx := targetX - originX
	dy := targetY - originY
	angle := math.Atan2(dy, dx)

	return Point{
		X: math.Cos(angle) * speed,
		Y: math.Sin(angle) * speed,
	}
}

// Example: When a player joins or leaves the lobby
func updateLobbyActivity(lobbyID string) {
	globalGameState.Lock()
	if lobby, ok := globalGameState.Lobbies[lobbyID]; ok {
		lobby.LastActivity = time.Now()
	}
	globalGameState.Unlock()
}

func StartLobbyCleanupTicker() {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	go func() {
		for range ticker.C {
			cleanupLobbies()
		}
	}()
}

func cleanupLobbies() {
	globalGameState.Lock()
	defer globalGameState.Unlock()

	for id, lobby := range globalGameState.Lobbies {
		if time.Since(lobby.LastActivity) > 10*time.Minute && len(lobby.Players) == 0 {
			// Lobby is inactive and has no players, remove it
			delete(globalGameState.Lobbies, id)
			fmt.Printf("Lobby %s removed due to inactivity\n", id)
		}
	}
}

func GameTick() {
	ticker := time.NewTicker(16 * time.Millisecond) // Approximately 60 ticks per second
	lastTick := time.Now()                          // Initialize lastTick to the current time

	canvasWidth := float64(2560)
	canvasHeight := float64(1440)

	// Define constants outside the loop to avoid recalculating them each tick
	const acceleration = 33.0
	const smoothing = 5.0
	const damage = 10.0

	go func() {
		for range ticker.C {
			now := time.Now()
			deltaTime := now.Sub(lastTick).Seconds() * 10
			lastTick = now

			globalGameState.Lock()

			// Iterate through all lobbies
			for _, lobby := range globalGameState.Lobbies {
				// Update each player's state
				for p := range lobby.Players {
					player := &lobby.Players[p]

					// Update target velocity based on key presses
					player.TargetVelocityY = 0
					if player.Controls.Up {
						player.TargetVelocityY = -acceleration
					} else if player.Controls.Down {
						player.TargetVelocityY = acceleration
					}

					player.TargetVelocityX = 0
					if player.Controls.Left {
						player.TargetVelocityX = -acceleration
					} else if player.Controls.Right {
						player.TargetVelocityX = acceleration
					}

					// Smoothly interpolate towards the target velocity
					player.VelocityY += (player.TargetVelocityY - player.VelocityY) * smoothing * deltaTime
					player.VelocityX += (player.TargetVelocityX - player.VelocityX) * smoothing * deltaTime

					// Update player position
					player.PositionX += player.VelocityX * deltaTime
					player.PositionY += player.VelocityY * deltaTime

					// Clamp PositionX and PositionY
					player.PositionX = clamp(player.PositionX, 0, canvasWidth)
					player.PositionY = clamp(player.PositionY, 0, canvasHeight)

					// Update player rotation angle towards the mouse
					player.Angle = calculateRotationAngle(player.PositionX, player.PositionY, player.MousePositionX, player.MousePositionY)

					// Handle collisions with projectiles
					indicesToRemove := map[int]bool{} // Store indices of projectiles to remove
					for j := range lobby.Projectiles {
						projectile := &lobby.Projectiles[j]

						if isCollision(*player, *projectile) {
							// Handle projectile hit
							player.Health -= damage
							fmt.Printf("Player %s hit! Health: %f\n", player.PlayerID, player.Health)

							// Mark projectile for removal
							indicesToRemove[j] = true

							// Handle player death
							if player.Health <= 0 {
								fmt.Printf("Player %s is dead!\n", player.PlayerID)
								// Implement player death logic here
							}
						}
					}

					// Remove the projectiles marked for deletion
					lobby.Projectiles = removeProjectiles(lobby.Projectiles, indicesToRemove)
				}

				// Update projectile positions and remove if off-screen
				indicesToRemove := map[int]bool{} // Store indices of projectiles to remove
				for j := range lobby.Projectiles {
					projectile := &lobby.Projectiles[j]

					// Update projectile position
					projectile.PositionX += projectile.VelocityX * deltaTime
					projectile.PositionY += projectile.VelocityY * deltaTime

					// Check if the projectile is off-screen
					if isProjectileOffScreen(projectile, canvasWidth, canvasHeight) {
						indicesToRemove[j] = true
					}
				}

				// Remove the off-screen projectiles
				lobby.Projectiles = removeProjectiles(lobby.Projectiles, indicesToRemove)

				// Broadcast updated game state to all players
				broadcastGameState(lobby)
			}

			globalGameState.Unlock()
		}
	}()
}

func broadcastGameState(lobby *GameState) {
	response := types.FrontendResponse{
		ID:   "game_update",
		Data: lobby,
	}
	broadcastMessageToGameRoom(lobby.GameID, response)
}

// Helper function to remove projectiles based on their indices
func removeProjectiles(projectiles []Projectile, indicesToRemove map[int]bool) []Projectile {
	newProjectiles := projectiles[:0] // Keep capacity, avoid memory reallocation
	for i, proj := range projectiles {
		if !indicesToRemove[i] {
			newProjectiles = append(newProjectiles, proj)
		}
	}
	return newProjectiles
}

// Check if a projectile is off-screen
func isProjectileOffScreen(proj *Projectile, canvasWidth, canvasHeight float64) bool {
	return proj.PositionX < 0 || proj.PositionX > canvasWidth || proj.PositionY < 0 || proj.PositionY > canvasHeight
}
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func calculateRotationAngle(triangleX, triangleY, mouseX, mouseY float64) float64 {
	dy := mouseY - triangleY
	dx := mouseX - triangleX
	// `math.Atan2` gives the angle from the x-axis, so subtract 90 degrees (in radians)
	// to align the triangle's top with the cursor
	// In Go, math.Pi represents π
	return math.Atan2(dy, dx) - math.Pi/2 + math.Pi
}

func broadcastMessageToGameRoom(gameID string, message types.FrontendResponse) {
	jsonResponse, err := json.Marshal(message)
	if err != nil {
		log.Println("Error marshalling JSON:", err)
		return
	}

	for _, safeConn := range activeConnections {
		safeConn.Mutex.Lock() // Lock the connection-specific mutex
		err := safeConn.Conn.WriteMessage(websocket.TextMessage, jsonResponse)
		safeConn.Mutex.Unlock() // Unlock the connection-specific mutex

		if err != nil {
			log.Println("Error writing to WebSocket:", err)
			removeConnection(safeConn)
		}
	}
}

func removeConnection(safeConn *SafeConnection) {
	connMutex.Lock()
	defer connMutex.Unlock()
	// Find the connection in the map and remove it
	for userID, conn := range activeConnections {
		if conn == safeConn {
			delete(activeConnections, userID)
			break
		}
	}
	// Safely close the connection
	safeConn.Conn.Close()
}

func rotateAndTranslate(point Point, angle, centerX, centerY float64) Point {
	// Precompute cosine and sine for the given angle
	cosAngle := math.Cos(angle)
	sinAngle := math.Sin(angle)

	// Rotate around the center (0, 0)
	rotatedX := cosAngle*point.X - sinAngle*point.Y
	rotatedY := sinAngle*point.X + cosAngle*point.Y

	// Then translate the point to its actual position
	return Point{
		X: rotatedX + centerX,
		Y: rotatedY + centerY,
	}
}

func calculateOffset(angle, distance, perpendicularDistance float64) Point {
	return Point{
		X: math.Cos(angle)*distance - math.Sin(angle)*perpendicularDistance,
		Y: math.Sin(angle)*distance + math.Cos(angle)*perpendicularDistance,
	}
}

func isCollision(player Player, projectile Projectile) bool {
	playerRadius := 20.0    // example radius for player
	projectileRadius := 5.0 // example radius for projectile

	dx := player.PositionX - projectile.PositionX
	dy := player.PositionY - projectile.PositionY
	distance := math.Sqrt(dx*dx + dy*dy)

	return distance < (playerRadius + projectileRadius)
}
