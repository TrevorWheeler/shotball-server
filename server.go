package main

import (
	"encoding/json"
	"fmt"
	"myapp/src/lobby"
	"myapp/src/types"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return r.Header.Get("Origin") == "http://localhost:8080"
		},
	}
)

func main() {
	lobby.StartLobbyCleanupTicker()
	lobby.GameTick()
	e := echo.New()
	e.Debug = true
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Static("/", "../public")
	e.GET("/ws", connect)
	e.Logger.Fatal(e.Start(":3000"))

}

// Connect function
func connect(c echo.Context) error {

	// WebSocket upgrade and other setup code...
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()
	for {
		messageType, msg, err := ws.ReadMessage()

		if messageType != websocket.TextMessage || messageType == websocket.TextMessage && string(msg) != "ping" {
			c.Logger().Info("Request: ", msg)
		}

		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				c.Logger().Error("Normal WebSocket closure detected.", err)
				break
			}
			c.Logger().Error("WebSocket read error:", err)
			continue
		}

		// TODO: ping pong - handle concurrent websocket write issue caused by pong response.
		// if messageType == websocket.TextMessage && string(msg) == "ping" {
		// 	err = ws.WriteMessage(websocket.TextMessage, []byte("pong"))
		// 	if err != nil {
		// 		c.Logger().Error("Error sending pong:", err)
		// 	}
		// 	continue
		// }

		var request types.FrontendRequest
		if err := json.Unmarshal([]byte(msg), &request); err != nil {
			c.Logger().Error("Error unmarshalling JSON:", err)
			continue
		}

		// Check if data is of type map[string]interface{}.
		requestData, ok := request.Data.(map[string]interface{})
		if !ok && request.Data != nil {
			c.Logger().Error("Data not of expected type.", nil)
			fmt.Printf("%T\n", request.Data)
			continue
		}

		switch request.ID {
		case "create_game":
			// Handle other types similarly based on different IDs
			if err := lobby.CreateGame(c, ws, requestData); err != nil {
				handleErrorAndCloseConnection(c, ws, err)
				return nil
			}
		case "join_game":
			// Handle other types similarly based on different IDs
			if err := lobby.JoinGame(c, ws, requestData); err != nil {
				handleErrorAndCloseConnection(c, ws, err)
				return nil
			}
		case "player_update_position":
			// Handle other types similarly based on different IDs
			if err := lobby.PlayerUpdatePosition(c, ws, requestData, request.Token); err != nil {
				handleErrorAndCloseConnection(c, ws, err)
				return nil
			}
		case "player_shoot_projectile":
			// Handle other types similarly based on different IDs
			if err := lobby.PlayerShootProjectile(c, ws, requestData, request.Token); err != nil {
				handleErrorAndCloseConnection(c, ws, err)
				return nil
			}
		default:
			fmt.Println("Unhandled ID")
		}
	}
	return nil
}

func handleErrorAndCloseConnection(c echo.Context, ws *websocket.Conn, err error) {
	if err == nil {
		return
	}
	c.Logger().Error(err)
	closeMessage := websocket.FormatCloseMessage(websocket.CloseNormalClosure, err.Error())
	if closeErr := ws.WriteMessage(websocket.CloseMessage, closeMessage); closeErr != nil {
		c.Logger().Error("Error sending close message:", closeErr)
	}
	if closeErr := ws.Close(); closeErr != nil {
		c.Logger().Error("Error closing WebSocket:", closeErr)
	}
}

type RequestHandler func(echo.Context, *websocket.Conn, map[string]interface{}) error

var handlers = map[string]RequestHandler{
	"create_game": lobby.CreateGame,
	"join_game":   lobby.JoinGame,
	// "player_update_position":  lobby.PlayerUpdatePosition,
	// "player_shoot_projectile": lobby.PlayerShootProjectile,
	// Add other handlers here
}

func handleWebSocketRequest(c echo.Context, ws *websocket.Conn, request types.FrontendRequest) {
	handler, found := handlers[request.ID]
	if !found {
		fmt.Println("Unhandled ID")
		return
	}

	requestData, ok := request.Data.(map[string]interface{})
	if !ok && request.Data != nil {
		c.Logger().Error("Data not of expected type.", nil)
		return
	}

	if err := handler(c, ws, requestData); err != nil {
		handleErrorAndCloseConnection(c, ws, err)
	}
}
