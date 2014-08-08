package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

var ignore = flag.String("ignore", "^(.git|.hg|.svn|_darcs|.bzr)$", "Ignore directory")

func main() {
	ignorere, err := regexp.Compile(*ignore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	base := "."
	if len(os.Args) > 1 {
		base = os.Args[1]
	}
	filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			if p, err := filepath.Abs(path); err == nil {
				fmt.Println(p)
			}
		} else {
			if ignorere.MatchString(info.Name()) {
				return filepath.SkipDir
			}
		}
		return nil
	})
}
