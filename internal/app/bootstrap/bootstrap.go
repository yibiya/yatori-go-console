package bootstrap

import (
	"fmt"

	"yatori-go-console/config"
	consoleinit "yatori-go-console/init"
	"yatori-go-console/logic"
	"yatori-go-console/utils"
)

type hooks struct {
	initConsole      func()
	printLogo        func()
	showAnnouncement func()
	launch           func()
}

// Run executes the legacy startup sequence behind a stable bootstrap entrypoint.
func Run() {
	runWithHooks(defaultHooks())
}

func defaultHooks() hooks {
	return hooks{
		initConsole: consoleinit.YatoriConsoleInit,
		printLogo: func() {
			fmt.Println(config.YaotirLogo())
		},
		showAnnouncement: utils.ShowAnnouncement,
		launch:           logic.Lunch,
	}
}

func runWithHooks(h hooks) {
	if h.initConsole != nil {
		h.initConsole()
	}
	if h.printLogo != nil {
		h.printLogo()
	}
	if h.showAnnouncement != nil {
		h.showAnnouncement()
	}
	if h.launch != nil {
		h.launch()
	}
}
