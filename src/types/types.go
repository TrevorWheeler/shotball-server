package types

import "github.com/golang-jwt/jwt/v5"

type FrontendRequest struct {
	Token string      `json:"token"`
	ID    string      `json:"id"`
	Data  interface{} `json:"data"`
}

type FrontendResponse struct {
	ID   string      `json:"id"`
	Data interface{} `json:"data"`
}

type LoginData struct {
	Username string `json:"username"`
}

type Token struct {
	PlayerID string `json:"playerId"`
	Username string `json:"username"`
	GameId   string `json:"gameId"`
	jwt.RegisteredClaims
}

type PlayerDirection struct {
	Up    bool `json:"up"`
	Down  bool `json:"down"`
	Left  bool `json:"left"`
	Right bool `json:"right"`
}
