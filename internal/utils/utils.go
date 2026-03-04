package utils

import "github.com/google/uuid"

func GenerateSessionID() (string, error) {
	res, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return res.String(), err
}
