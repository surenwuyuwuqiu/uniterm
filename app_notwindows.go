//go:build !windows

package main

func (a *App) findMainWindow() uintptr { return 0 }

func (a *App) subclassMainWindow() {}

func (a *App) unsubclassMainWindow() {}
