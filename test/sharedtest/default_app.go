package sharedtest

import (
	"sync"
	"testing"
)

var makeDefaultAppOnce sync.Once
var defaultTestApp *App

func GetDefaultTestApp() *App {
	makeDefaultAppOnce.Do(func() {
		defaultTestApp = RunApp()
	})
	return defaultTestApp
}

func Login(t *testing.T) *User {
	return GetDefaultTestApp().Login(t)
}
