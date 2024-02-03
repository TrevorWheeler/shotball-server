package authentication

import (
	"fmt"
	"log"
	"myapp/src/types"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

func GenerateToken(username string, gameId string, playerId string) (string, error) {
	if username == "" {
		return "", fmt.Errorf("username is empty")
	}

	// Set custom claims
	claims := &types.Token{
		PlayerID: playerId,
		Username: username,
		GameId:   gameId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)),
		},
	}

	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file")
		return "", err
	}

	secret := os.Getenv("SECRET")

	// Generate encoded token
	signedToken, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

func ParseToken(tokenString string) (*jwt.Token, *types.Token, error) {
	claims := &types.Token{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Make sure that the token's signing method corresponds to the signing method you used
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Return the secret key
		secret := os.Getenv("SECRET")
		return []byte(secret), nil
	})

	if err != nil {
		return nil, nil, err
	}

	return token, claims, nil
}
