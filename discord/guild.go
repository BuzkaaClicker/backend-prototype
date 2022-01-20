package discord

import (
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
)

type GuildAddStatus int

const (
	GuildAddStatusSuccess       GuildAddStatus = 201
	GuildAddStatusAlreadyMember GuildAddStatus = 204
)

type GuildMemberAdd = func(userAccessToken string, userId string) (GuildAddStatus, error)

func MockGuildMemberAdd(userAccessToken string, userId string) (GuildAddStatus, error) {
	return GuildAddStatusSuccess, nil
}

// Impl of discord rest api /guilds/{guild.id}/members/{user.id}
func RestGuildMemberAdd(botToken string, guildId string) GuildMemberAdd {
	return func(userAccessToken string, userId string) (GuildAddStatus, error) {
		agent := fiber.AcquireAgent()
		defer fiber.ReleaseAgent(agent)

		req := agent.Request()
		req.Header.SetMethod(fiber.MethodPut)
		req.Header.Set(fiber.HeaderAuthorization, "Bot "+botToken)
		req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
		req.SetRequestURI(fmt.Sprintf("https://discord.com/api/guilds/%s/members/%s",
			url.PathEscape(guildId), url.PathEscape(userId)))

		type ReqBody struct {
			AccessToken string `json:"access_token"`
		}
		reqBody, err := json.Marshal(ReqBody{AccessToken: userAccessToken})
		if err != nil {
			return 0, fmt.Errorf("marshal body: %w", err)
		}
		req.SetBody(reqBody)

		err = agent.Parse()
		if err != nil {
			return 0, fmt.Errorf("agent parse: %w", err)
		}

		statusCode, body, errs := agent.Bytes()
		if errs != nil {
			return 0, fmt.Errorf("agent bytes: %v", errs)
		}
		if statusCode != fiber.StatusCreated && statusCode != fiber.StatusNoContent {
			if statusCode == fiber.StatusUnauthorized {
				return 0, ErrUnauthorized
			} else {
				return 0, fmt.Errorf("invalid status code %d: %s", statusCode, string(body))
			}
		}
		return GuildAddStatus(statusCode), nil
	}
}
