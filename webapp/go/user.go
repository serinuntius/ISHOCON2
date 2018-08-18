package main

import (
	"context"

	"github.com/pkg/errors"
)

// User Model
type User struct {
	ID         int
	Name       string
	Address    string
	MyNumber   string
	Votes      int
	VotedCount int
}

func getUser(ctx context.Context, name string, address string, myNumber string) (*User, error) {
	user := User{}
	row := db.QueryRowContext(ctx, "SELECT * FROM users WHERE mynumber = ?", myNumber)
	if err := row.Scan(&user.ID, &user.Name, &user.Address, &user.MyNumber, &user.Votes, &user.VotedCount); err != nil {
		return nil, err
	}

	if user.Name != name || user.Address != address {
		return nil, errors.New("user not found")
	}

	return &user, nil
}
