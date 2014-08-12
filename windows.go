package main

import (
	"fmt"
	"os"
	"syscall"
	"unicode/utf16"
)

const maxWrite = 16000

func init() {
	printLine = func(a ...interface{}) (n int, err error) {
		runes := []rune(fmt.Sprint(a...))
		f := syscall.Handle(os.Stdout.Fd())
		for len(runes) > 0 {
			m := len(runes)
			if m > maxWrite {
				m = maxWrite
			}
			chunk := runes[:m]
			runes = runes[m:]
			uint16s := utf16.Encode(chunk)
			for len(uint16s) > 0 {
				var written uint32
				err = syscall.WriteConsole(f, &uint16s[0], uint32(len(uint16s)), &written, nil)
				if err != nil {
					return 0, nil
				}
				uint16s = uint16s[written:]
			}
		}
		return n, nil
	}
}
