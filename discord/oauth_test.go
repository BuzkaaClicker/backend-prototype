package discord

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateDiscordOauthUrl(t *testing.T) {
	assert := assert.New(t)

	cases := []struct {
		clientId    string
		redirectUri string
		result      string
	}{
		{"2115", "https://buzkaaclicker.pl/discord_login", "https://discord.com/api/oauth2/authorize?client_id=2115&" +
			"redirect_uri=https%3A%2F%2Fbuzkaaclicker.pl%2Fdiscord_login&response_type=code&scope=email+identify+guilds.join"},
		{"3721", "https://buzkaaclicker.pl/discord", "https://discord.com/api/oauth2/authorize?client_id=3721&" +
			"redirect_uri=https%3A%2F%2Fbuzkaaclicker.pl%2Fdiscord&response_type=code&scope=email+identify+guilds.join"},
	}

	for i, tc := range cases {
		f := RestOAuthUrlFactory(tc.clientId, tc.redirectUri)
		assert.Equal(tc.result, f(), "index: %d", i)
	}
}
