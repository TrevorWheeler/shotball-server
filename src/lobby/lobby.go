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
	// Connection map
	activeConnections = make(map[string]*SafeConnection)
	// Mutex to ensure thread safety for the connection map
	connMutex sync.Mutex
)

func addConnection(userID string, conn *websocket.Conn) {
	connMutex.Lock()
	defer connMutex.Unlock()
	activeConnections[userID] = &SafeConnection{Conn: conn}
}

var globalGameState = struct {
	sync.RWMutex
	Lobbies map[string]*GateState
}{
	Lobbies: make(map[string]*GateState),
}

type GateState struct {
	GameID       string       `json:"gameId"`
	Players      []Player     `json:"players"`
	Projectiles  []Projectile `json:"projectiles"`
	LastActivity time.Time
}

type Player struct {
	PlayerID        string                `json:"playerId"`
	Username        string                `json:"username"`
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
	newLobby := &GateState{
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
	LobbyId string `json:"lobbyId"`
	// other fields
}

type LobbyResponse struct {
	LobbyID string   `json:"lobbyId"`
	Players []Player `json:"players"`
}

func JoinGame(c echo.Context, ws *websocket.Conn, requestData map[string]interface{}) error {
	gameId, ok := requestData["lobbyId"].(string)
	if !ok {
		return fmt.Errorf("lobby id not provided")
	}
	playerId := uuid.New().String()
	c.Logger().Info(gameId)
	c.Logger().Info(playerId)

	username, ok := requestData["username"].(string)
	if !ok || username == "" {
		return fmt.Errorf("username not provided")
	}

	signedToken, err := authentication.GenerateToken(username, gameId, playerId)
	if err != nil {
		log.Println("Error getting token:", err)
		return fmt.Errorf("failed to generate user token")
	}

	player := Player{
		PlayerID:        playerId,
		Username:        username,
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

	if lobby, ok := globalGameState.Lobbies[gameId]; ok {
		lobby.Players = append(lobby.Players, player)
		response := types.FrontendResponse{
			ID: "game_enter",
			Data: FrontendGameEnter{
				Token: signedToken,
				GameState: FrontendGameState{
					GameID:      gameId,
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

	// Don't forget to unlock after you're done with the modification
	globalGameState.Unlock()
	updateLobbyActivity(gameId)
	return nil
}

func PlayerUpdatePosition(c echo.Context, ws *websocket.Conn, requestData map[string]interface{}, tokenString string) error {
	token, claims, err := authentication.ParseToken(tokenString)
	if err != nil {
		// Handle the error, perhaps return an HTTP 401 Unauthorized status
		return err
	}
	if !token.Valid {
		// Token is not valid
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

	// Use the values from claims
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
		// Handle the error, perhaps return an HTTP 401 Unauthorized status
		return err
	}
	if !token.Valid {
		// Token is not valid
		return fmt.Errorf("invalid token")
	}
	// gameId, ok := requestData["gameId"].(bool)
	// if !ok {
	// 	return fmt.Errorf("up not provided")
	// }

	// Use the values from claims
	playerId := claims.PlayerID
	gameId := claims.GameId

	globalGameState.Lock()
	if lobby, ok := globalGameState.Lobbies[gameId]; ok {

		// Iterate through players to find the matching one
		for i := range lobby.Players {
			if lobby.Players[i].PlayerID == playerId {

				var angle float64 = lobby.Players[i].Angle
				var x float64 = lobby.Players[i].PositionX
				var y float64 = lobby.Players[i].PositionY

				topVertex := rotateAndTranslate(Point{X: 0, Y: -20}, angle, x, y)

				var distance float64 = float64(0)
				var perpendicularDistance float64 = float64(-5)

				offset := calculateOffset(angle, distance, perpendicularDistance)

				startPositionX := topVertex.X + offset.X

				startPositionY := topVertex.Y + offset.Y

				// // Calculate the velocity of the projectile towards the mouse position
				projectileVelocity := calculateProjectileVelocity(
					startPositionX,
					startPositionY,
					lobby.Players[i].MousePositionX,
					lobby.Players[i].MousePositionY,
					float64(13),
				)

				projectile := Projectile{
					ProjectileID: uuid.New().String(),
					PlayerID:     playerId,
					PositionX:    startPositionX,
					PositionY:    startPositionY,
					VelocityX:    projectileVelocity.X,
					VelocityY:    projectileVelocity.Y,
				}

				lobby.Projectiles = append(lobby.Projectiles, projectile)

				// broadcastMessageToGameRoom(gameId, response)

				break
			} else {
				fmt.Printf("notFound")
			}
		}
	} else {
		c.Logger().Error("Problem")
	}

	globalGameState.Unlock()

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
	// ticker := time.NewTicker(1 * time.Second) // Approximately 60 ticks per second
	lastTick := time.Now() // Initialize lastTick to the current time

	canvasWidth := float64(2560)
	canvasHeight := float64(1440)

	go func() {
		for range ticker.C {
			// Calculate delta time
			now := time.Now()
			// fmt.Printf("############### \n")
			// fmt.Printf("now : %+v\n", now)
			// fmt.Printf("lastTick : %+v\n", lastTick)
			deltaTime := now.Sub(lastTick).Seconds() * 10
			lastTick = now

			globalGameState.Lock()
			// Iterate through players to find the matching one
			for i := range globalGameState.Lobbies {
				acceleration := float64(33)
				smoothing := float64(5)
				for p := range globalGameState.Lobbies[i].Players {

					// Update target velocity based on key presses
					if globalGameState.Lobbies[i].Players[p].Controls.Up {
						globalGameState.Lobbies[i].Players[p].TargetVelocityY = -acceleration

					} else if globalGameState.Lobbies[i].Players[p].Controls.Down {
						globalGameState.Lobbies[i].Players[p].TargetVelocityY = acceleration
					} else {
						globalGameState.Lobbies[i].Players[p].TargetVelocityY = 0
					}

					if globalGameState.Lobbies[i].Players[p].Controls.Left {
						globalGameState.Lobbies[i].Players[p].TargetVelocityX = -acceleration
					} else if globalGameState.Lobbies[i].Players[p].Controls.Right {
						globalGameState.Lobbies[i].Players[p].TargetVelocityX = acceleration
					} else {
						globalGameState.Lobbies[i].Players[p].TargetVelocityX = 0
					}

					// Smoothly interpolate towards the target velocity
					globalGameState.Lobbies[i].Players[p].VelocityY += (globalGameState.Lobbies[i].Players[p].TargetVelocityY - globalGameState.Lobbies[i].Players[p].VelocityY) * smoothing * deltaTime

					globalGameState.Lobbies[i].Players[p].VelocityX += (globalGameState.Lobbies[i].Players[p].TargetVelocityX - globalGameState.Lobbies[i].Players[p].VelocityX) * smoothing * deltaTime

					globalGameState.Lobbies[i].Players[p].PositionX += globalGameState.Lobbies[i].Players[p].VelocityX * deltaTime
					globalGameState.Lobbies[i].Players[p].PositionY += globalGameState.Lobbies[i].Players[p].VelocityY * deltaTime
					// Clamp PositionX and PositionY
					globalGameState.Lobbies[i].Players[p].PositionX = clamp(globalGameState.Lobbies[i].Players[p].PositionX, 0, canvasWidth)
					globalGameState.Lobbies[i].Players[p].PositionY = clamp(globalGameState.Lobbies[i].Players[p].PositionY, 0, canvasHeight)

					globalGameState.Lobbies[i].Players[p].Angle =
						calculateRotationAngle(
							globalGameState.Lobbies[i].Players[p].PositionX,
							globalGameState.Lobbies[i].Players[p].PositionY,
							globalGameState.Lobbies[i].Players[p].MousePositionX,
							globalGameState.Lobbies[i].Players[p].MousePositionY,
						)

				}

				for j := len(globalGameState.Lobbies[i].Projectiles) - 1; j >= 0; j-- {
					// Update projectile position
					globalGameState.Lobbies[i].Projectiles[j].PositionX += globalGameState.Lobbies[i].Projectiles[j].VelocityX * deltaTime
					globalGameState.Lobbies[i].Projectiles[j].PositionY += globalGameState.Lobbies[i].Projectiles[j].VelocityY * deltaTime

					// Check if the projectile is off-screen
					withinVerticalBounds := globalGameState.Lobbies[i].Projectiles[j].PositionY >= 0 && globalGameState.Lobbies[i].Projectiles[j].PositionY <= canvasHeight
					withinHorizontalBounds := globalGameState.Lobbies[i].Projectiles[j].PositionX >= 0 && globalGameState.Lobbies[i].Projectiles[j].PositionX <= canvasWidth

					// Remove the projectile if it's off-screen
					if !(withinVerticalBounds && withinHorizontalBounds) {
						// Remove the projectile by swapping it with the last one and trimming the slice
						globalGameState.Lobbies[i].Projectiles[j] = globalGameState.Lobbies[i].Projectiles[len(globalGameState.Lobbies[i].Projectiles)-1]
						globalGameState.Lobbies[i].Projectiles = globalGameState.Lobbies[i].Projectiles[:len(globalGameState.Lobbies[i].Projectiles)-1]
					}
				}

				response := types.FrontendResponse{
					ID:   "game_update",
					Data: globalGameState.Lobbies[i],
				}

				broadcastMessageToGameRoom(globalGameState.Lobbies[i].GameID, response)

				// fmt.Printf("DeltaTime : %+v\n", deltaTime)
				// fmt.Printf("VelocityY: %+v\n", globalGameState.Lobbies[i].Players[p].VelocityY)
				// fmt.Printf("Position Y: %+v\n", globalGameState.Lobbies[i].Players[p].PositionY)
				// fmt.Printf("VelocityX: %+v\n", globalGameState.Lobbies[i].Players[p].VelocityX)
				// fmt.Printf("Position X: %+v\n", globalGameState.Lobbies[i].Players[p].PositionX)
				// fmt.Printf("Position Y: %+v\n", globalGameState.Lobbies[i].Players[p].PositionY)
				// fmt.Printf("Position X: %+v\n", globalGameState.Lobbies[i].Players[p].PositionX)
				// fmt.Printf("############### \n")
			}

			globalGameState.Unlock()
		}
	}()
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
	// Rotate around the center (0, 0)
	rotatedX := math.Cos(angle)*point.X - math.Sin(angle)*point.Y
	rotatedY := math.Sin(angle)*point.X + math.Cos(angle)*point.Y

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