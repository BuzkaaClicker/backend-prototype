package discord

import (
	"encoding/json"
	"fmt"

	"github.com/gofiber/fiber/v2"
)

type User struct {
	Id       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type UserMe = func(tokenType string, token string) (User, error)

type UserMeProvider = func() UserMe

// Impl of discord rest api /user/@me
func RestUserMe(tokenType string, token string) (User, error) {
	agent := fiber.AcquireAgent()
	defer fiber.ReleaseAgent(agent)

	req := agent.Request()
	req.Header.SetMethod(fiber.MethodGet)
	req.SetRequestURI("https://discord.com/api/users/@me")

	req.Header.Set("Authorization", tokenType+" "+token)

	err := agent.Parse()
	if err != nil {
		return User{}, fmt.Errorf("agent parse: %w", err)
	}

	statusCode, body, errs := agent.Bytes()
	if errs != nil {
		return User{}, fmt.Errorf("agent bytes: %v", errs)
	}

	if statusCode != fiber.StatusOK {
		if statusCode == fiber.StatusUnauthorized {
			return User{}, ErrUnauthorized
		} else {
			return User{}, fmt.Errorf("invalid status code %d: %s", statusCode, string(body))
		}
	}

	var response User
	if err = json.Unmarshal(body, &response); err != nil {
		return User{}, fmt.Errorf("unmarshal body: %w", err)
	}
	return response, nil
}

func RestUserMeProvider() UserMe {
	return RestUserMe
}
