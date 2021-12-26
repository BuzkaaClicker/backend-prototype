package discord

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
)

var ErrOAuthInvalidCode = errors.New("discord: oauth invalid code")

type OAuthUrlFactory = func() string

type AccessTokenExchange = func(code string) (AccessTokenResponse, error)

type AccessTokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

func RestOAuthUrlFactory(clientId string, redirectUri string) OAuthUrlFactory {
	return func() string {
		loginUrl, err := url.Parse("https://discord.com/api/oauth2/authorize")
		if err != nil {
			logrus.WithError(err).Fatalln("Could not parse discord login url.")
			return ""
		}
		query := loginUrl.Query()
		query.Set("client_id", clientId)
		query.Set("redirect_uri", redirectUri)
		query.Set("response_type", "code")
		query.Set("scope", "email identify guilds.join")
		loginUrl.RawQuery = query.Encode()
		return loginUrl.String()
	}
}

func RestAccessTokenExchanger(clientId string, clientSecret string, redirectUri string) AccessTokenExchange {
	return func(code string) (AccessTokenResponse, error) {
		agent := fiber.AcquireAgent()
		defer fiber.ReleaseAgent(agent)

		req := agent.Request()
		req.Header.SetMethod(fiber.MethodPost)
		req.SetRequestURI("https://discord.com/api/oauth2/token")

		args := fiber.AcquireArgs()
		defer fiber.ReleaseArgs(args)

		args.Add("grant_type", "authorization_code")
		args.Add("client_id", clientId)
		args.Add("client_secret", clientSecret)
		args.Add("code", code)
		args.Add("redirect_uri", redirectUri)

		err := agent.Form(args).Parse()
		if err != nil {
			return AccessTokenResponse{}, fmt.Errorf("agent parse: %w", err)
		}

		statusCode, bodyBytes, errArr := agent.Bytes()
		if len(errArr) != 0 {
			return AccessTokenResponse{}, fmt.Errorf("agent bytes: %v", errArr)
		}
		if statusCode != fiber.StatusOK {
			return accessTokenExchangeError(statusCode, bodyBytes)
		}

		var response AccessTokenResponse
		err = json.Unmarshal(bodyBytes, &response)
		if err != nil {
			return AccessTokenResponse{}, fmt.Errorf("response unmarshal: %w", err)
		}
		return response, nil
	}
}

func accessTokenExchangeError(statusCode int, bodyBytes []byte) (AccessTokenResponse, error) {
	type ErrorResponse struct {
		Description string `json:"error_description"`
	}
	var response ErrorResponse
	err := json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return AccessTokenResponse{}, fmt.Errorf("unmarshal error: %w: %s",
			err, string(bodyBytes))
	}

	if response.Description == `Invalid "code" in request.` {
		return AccessTokenResponse{}, ErrOAuthInvalidCode
	} else {
		return AccessTokenResponse{}, fmt.Errorf("invalid status code '%d': %s",
			statusCode, string(bodyBytes))
	}
}
