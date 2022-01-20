package discord

import "errors"

var ErrUnauthorized = errors.New("discord: unauthorized")

type Token struct {
	Type  string
	Value string
}

func (t Token) String() string {
	return t.Type + " " + t.Value
}
