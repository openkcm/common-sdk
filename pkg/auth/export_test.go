package auth

import "time"

func SetNowFunc(newNowFunc func() time.Time) {
	nowFunc = newNowFunc
}
