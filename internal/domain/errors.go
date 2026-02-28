package domain

import "errors"

var (
	ErrAccountNotFound = errors.New("account not found")
	ErrPoolNotFound    = errors.New("pool not found")
	ErrSecretNotFound  = errors.New("secret not found")
)
