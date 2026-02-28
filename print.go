package sylph

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	C "github.com/fatih/color"
)

var (
	enabled bool
	// filter       = C.New(C.BgYellow, C.Bold)
	forceDisable bool
)

func EnabledDebug() bool {
	return enabled
}

func DisableDebug() {
	enabled = false
}

func ForceDisableDebug(disable bool) {
	forceDisable = disable
}

func EnableDebug() {
	enabled = true
}

func base(force bool, kind string, format string, a ...any) {
	if !force && !enabled {
		return
	}

	if force {
		if forceDisable {
			return
		}
	}

	switch kind {
	case "black":
		C.Black(format, a...)
	case "red":
		C.Red(format, a...)
	case "green":
		C.Green(format, a...)
	case "yellow":
		C.Yellow(format, a...)
	case "blue":
		C.Blue(format, a...)
	case "magenta":
		C.Magenta(format, a...)
	case "cyan":
		C.Cyan(format, a...)
	case "white":
		C.White(format, a...)
	}

	C.Unset()
}

// func Black(format string, a ...any) {
// 	base(false, "black", format, a...)
// }

func printRed(format string, a ...any) {
	base(false, "red", format, a...)
}

func printGreen(format string, a ...any) {
	base(false, "green", format, a...)
}

func printYellow(format string, a ...any) {
	base(false, "yellow", format, a...)
}

// func Blue(format string, a ...any) {
// 	base(false, "blue", format, a...)
// }

func printMagenta(format string, a ...any) {
	base(false, "magenta", format, a...)
}

func printCyan(format string, a ...any) {
	base(false, "cyan", format, a...)
}

// func White(format string, a ...any) {
// 	base(false, "white", format, a...)
// }

func forceRed(format string, a ...any) {
	base(true, "red", format, a...)
}

func forceGreen(format string, a ...any) {
	base(true, "green", format, a...)
}

func forceYellow(format string, a ...any) {
	base(true, "yellow", format, a...)
}

func forceBlue(format string, a ...any) {
	base(true, "blue", format, a...)
}

func forceMagenta(format string, a ...any) {
	base(true, "magenta", format, a...)
}

func forceCyan(format string, a ...any) {
	base(true, "cyan", format, a...)
}

func printPanic(format string, a ...any) {
	s := fmt.Sprintf(format, a...)
	panic(s)
}

// func RedPanic(format string, a ...any) {
// 	s := SRed(format, a...)
// 	panic(s)
// }

// 系统类: Cyan
// 错误类: Red
// 提示类: Magenta
// 强提示: Yellow
// 成功: Green

func prefix(mark, format string) string {
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	mark = fmt.Sprintf("[%s]", mark)
	return fmt.Sprintf("%23s %9s: %s", ts, mark, format)

}

func Mark(format string, a ...any) {
	format = prefix("mark", format)
	forceRed(format, a...)
	os.Exit(0)
}

func printSystem(format string, a ...any) {
	format = prefix("system", format)
	forceCyan(format, a...)
}

func printError(format string, a ...any) {
	format = prefix("error", format)
	forceRed(format, a...)
}

// func printNotice(format string, a ...any) {
// 	format = prefix("notice", format)
// 	printMagenta(format, a...)
// }

func printWarning(format string, a ...any) {
	format = prefix("warning", format)
	printYellow(format, a...)
}

func printInfo(format string, a ...any) {
	format = prefix("info", format)
	printGreen(format, a...)
}

func printJson(obj any) {
	if !enabled {
		return
	}
	js, _ := json.Marshal(obj)
	base(false, "red", string(js))
}
