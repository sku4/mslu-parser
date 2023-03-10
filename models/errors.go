package models

import "errors"

var (
	ArticlesNotFoundError = errors.New("articles not found")
	ProfileNotInitError   = errors.New("profile not init")
)
