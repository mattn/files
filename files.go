package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
)

var ignore = flag.String("i", "^(.git|.hg|.svn|_darcs|.bzr)$", "Ignore directory")
var progress = flag.Bool("p", false, "Progress message")
var async = flag.Bool("async", false, "Asynchronized")

var ignorere *regexp.Regexp

var printLine = fmt.Println

func filesSync(base string) {
	n := 0
	err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return err
		}
		if !info.IsDir() {
			if *progress {
				n++
				if n % 10 == 0 {
					fmt.Fprintf(os.Stderr, "\r%d            \r", n)
				}
			}
			fmt.Println(filepath.ToSlash(path[len(base)+1:]))
		} else {
			if ignorere.MatchString(info.Name()) {
				return filepath.SkipDir
			}
		}
		return nil
	})

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func filesAsync(base string) {
	wg := new(sync.WaitGroup)

	runtime.GOMAXPROCS(runtime.NumCPU())

	q := make(chan string, 20)

	var fn func(p string)
	fn = func(p string) {
		defer wg.Done()

		f, err := os.Open(p)
		if err != nil {
			return
		}
		defer f.Close()

		fis, err := f.Readdir(-1)
		if err != nil {
			return
		}
		for _, fi := range fis {
			if ignorere.MatchString(fi.Name()) {
				continue
			}
			if fi.IsDir() {
				wg.Add(1)
				go fn(filepath.Join(p, fi.Name()))
			} else {
				q <- filepath.Join(p, fi.Name())[len(base)+1:]
			}
		}
	}

	wg.Add(1)
	fi, err := os.Lstat(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "%q is not a directory")
		os.Exit(1)
	}
	go fn(base)

	go func() {
		wg.Wait()
		close(q)
	}()

	n := 0
	if *progress {
		for p := range q {
			n++
			if n%10 == 0 {
				fmt.Fprintf(os.Stderr, "\r%d            \r", n)
			}
			fmt.Println(p)
		}
	} else {
		for p := range q {
			fmt.Println(p)
		}
	}
}

func main() {
	flag.Parse()

	var err error
	ignorere, err = regexp.Compile(*ignore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	base := "."
	if flag.NArg() > 0 {
		base = flag.Arg(0)
	}

	base, err = filepath.Abs(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *async {
		filesAsync(base)
	} else {
		filesSync(base)
	}
}
