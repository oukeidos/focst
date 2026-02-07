package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"

	"github.com/oukeidos/focst/internal/logger"
)

func withPanicGuard(scope string, onPanic func(any), fn func()) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered panic", "scope", scope, "panic", fmt.Sprint(r))
			if onPanic != nil {
				onPanic(r)
			}
		}
	}()
	fn()
}

func safeGo(scope string, fn func()) {
	go func() {
		withPanicGuard(scope, nil, fn)
	}()
}

func safeDo(scope string, fn func()) {
	withPanicGuard(scope+".dispatch", nil, func() {
		fyne.Do(func() {
			withPanicGuard(scope, nil, fn)
		})
	})
}

func (a *focstApp) safeGo(scope string, fn func()) {
	if a == nil {
		safeGo(scope, fn)
		return
	}
	go func() {
		withPanicGuard(scope, func(r any) {
			a.handleRecoveredPanic(scope, r)
		}, fn)
	}()
}

func (a *focstApp) safeDo(scope string, fn func()) {
	if a == nil {
		safeDo(scope, fn)
		return
	}
	withPanicGuard(scope+".dispatch", func(r any) {
		a.handleRecoveredPanic(scope+".dispatch", r)
	}, func() {
		fyne.Do(func() {
			withPanicGuard(scope, func(r any) {
				a.handleRecoveredPanic(scope, r)
			}, fn)
		})
	})
}

func (a *focstApp) handleRecoveredPanic(scope string, _ any) {
	if a == nil {
		return
	}
	if fyne.CurrentApp() == nil {
		return
	}
	a.cancelActive("panic recovered: " + scope)
	a.setState(StateFailure)

	a.panicNoticeOnce.Do(func() {
		a.safeDo("panic.notice", func() {
			if a.window == nil {
				return
			}
			dialog.ShowInformation(
				"Unexpected Error",
				"An internal error occurred and the current task was stopped for safety. Please retry. If this repeats, restart the app.",
				a.window,
			)
		})
	})
}
