package session

import "errors"

var (
	ErrNoSession        = errors.New("session not found")
	ErrSessionExpired   = errors.New("session: expired")
	ErrSessionNotFound  = errors.New("session: not found")
	ErrSessionCreateErr = errors.New("session: failed to create")
	ErrSessionUpdateErr = errors.New("session: failed to update")
	ErrSessionDeleteErr = errors.New("session: failed to delete")
)
