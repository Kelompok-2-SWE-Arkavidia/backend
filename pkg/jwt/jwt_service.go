package jwt

import (
	"Go-Starter-Template/domain"
	"Go-Starter-Template/internal/utils"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"log"
	"time"
)

type (
	JWTService interface {
		GenerateTokenUser(userId string, role string) string
		ValidateTokenUser(token string) (*jwt.Token, error)
		GetUserIDByToken(token string) (string, string, error)
		GenerateTokenForgetPassword(data map[string]any, duration time.Duration) (string, error)
		ValidateTokenForgetPassword(token string) (jwt.MapClaims, error)
	}

	jwtUserClaim struct {
		UserID string `json:"user_id"`
		Role   string `json:"role"`
		jwt.RegisteredClaims
	}

	jwtService struct {
		secretKey string
		issuer    string
	}
)

func getSecretKey() string {
	utils.LoadConfig()
	secretKey := utils.GetConfig("JWT_SECRET")
	return secretKey
}

func NewJWTService() JWTService {
	return &jwtService{
		secretKey: getSecretKey(),
		issuer:    "FOODIA",
	}
}

func (j *jwtService) GenerateTokenUser(userId string, role string) string {
	claims := jwtUserClaim{
		userId,
		role,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 120)),
			Issuer:    j.issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tx, err := token.SignedString([]byte(j.secretKey))
	if err != nil {
		log.Println(err)
	}
	return tx
}

func (j *jwtService) parseToken(t_ *jwt.Token) (any, error) {
	if _, ok := t_.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method %v", t_.Header["alg"])
	}
	return []byte(j.secretKey), nil
}

func (j *jwtService) ValidateTokenUser(token string) (*jwt.Token, error) {
	return jwt.ParseWithClaims(token, &jwtUserClaim{}, j.parseToken)
}

func (j *jwtService) GetUserIDByToken(token string) (string, string, error) {
	t_Token, err := j.ValidateTokenUser(token)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", domain.ErrTokenExpired
		}
		return "", "", domain.ErrTokenInvalid
	}
	if !t_Token.Valid {
		return "", "", domain.ErrTokenInvalid
	}

	claims := t_Token.Claims.(*jwtUserClaim)

	id := fmt.Sprintf("%v", claims.UserID)
	role := fmt.Sprintf("%v", claims.Role)
	return id, role, nil
}

func (j *jwtService) GenerateTokenForgetPassword(data map[string]any, duration time.Duration) (string, error) {
	claims := jwt.MapClaims{}

	for key, value := range data {
		claims[key] = value
	}

	claims["exp"] = time.Now().Add(duration).Unix()
	claims["iat"] = time.Now().Unix()
	claims["iss"] = j.issuer

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(j.secretKey))
}

func (j *jwtService) ValidateTokenForgetPassword(token string) (jwt.MapClaims, error) {
	t_Token, err := j.ValidateTokenUser(token)
	if err != nil {
		return jwt.MapClaims{}, domain.ErrTokenExpired
	}

	if !t_Token.Valid {
		return jwt.MapClaims{}, domain.ErrTokenInvalid
	}

	claims := t_Token.Claims.(jwt.MapClaims)
	return claims, nil
}
