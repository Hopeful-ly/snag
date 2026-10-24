package platform

import "errors"

// ErrNoStreamPlayer means there is no default player that accepts a stream address.
var ErrNoStreamPlayer = errors.New("the default player can't open a stream")

// Player is an installed video player that plays a stream from its address.
type Player struct {
	ID      string `json:"id"` // how PlayStreamWith knows it
	Name    string `json:"name"`
	Default bool   `json:"default"`
}
