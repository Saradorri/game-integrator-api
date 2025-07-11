package app

import "github.com/saradorri/gameintegrator/internal/infrastructure/lock"

func (a *application) InitUserLockManager() *lock.UserLockManager {
	return lock.NewUserLockManager()
}
