package dots

import (
	"context"
	"log"
)

type key int

const (
	userContextKey = key(iota + 1)
	flashContextKey
)

func NewContextWithUser(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, userContextKey, u)
}

func UserFromContext(ctx context.Context) *User {
	u, ok := ctx.Value(userContextKey).(*User)
	if !ok {
		log.Printf("dots: user not found in context: %v\n", u)
		return &User{}
	}

	return u
}

type touristKey string

const touristContextKey touristKey = "channelTouristKey"

func NewContextWithTourist(ctx context.Context) (func(), context.Context) {
	ch := make(chan string, 1)
	cancel := func() {
		close(ch)
	}
	return cancel, context.WithValue(ctx, touristContextKey, ch)
}

func TouristFromContext(ctx context.Context) chan string {
	ch, ok := ctx.Value(touristContextKey).(chan string)
	if !ok {
		log.Printf("dots: tourist channel not found in context: %v\n", ch)
	}

	return ch
}
