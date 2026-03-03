package domain

import "errors"

// ErrQuit signals the user chose to quit.
var ErrQuit = errors.New("user quit")

// ErrGoBack signals the user chose to go back to the previous menu.
var ErrGoBack = errors.New("go back")
