package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"sync"
)

var ignore = flag.String("i", `^(\.git|\.hg|\.svn|_darcs|\.bzr)$`, "Ignore directory")
var progress = flag.Bool("p", false, "Progress message")
var async = flag.Bool("A", false, "Asynchronized")
var absolute = flag.Bool("a", false, "Absolute path")
var fsort = flag.Bool("s", false, "Sort")

var ignorere *regexp.Regexp

var printLine = fmt.Println

var printPath = func(path string) {
	p, err := filepath.Abs(path)
	if err == nil {
		path = p
	}
	printLine(path)
}

func filesSync(base string) chan string {
	q := make(chan string, 20)

	go func() {
		n := 0
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if info == nil {
				return err
			}
			if !info.IsDir() {
				if ignorere.MatchString(info.Name()) {
					return nil
				}
				if *progress {
					n++
					if n%10 == 0 {
						fmt.Fprintf(os.Stderr, "\r%d            \r", n)
					}
				}

				q <- filepath.ToSlash(path)
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

		close(q)
	}()

	return q
}

func filesAsync(base string) chan string {
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
				q <- filepath.ToSlash(filepath.Join(p, fi.Name()))
			}
		}
	}

	fi, err := os.Lstat(base)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if !fi.IsDir() {
		fmt.Fprintf(os.Stderr, "%q is not a directory")
		os.Exit(1)
	}

	wg.Add(1)
	go fn(base)

	go func() {
		wg.Wait()
		close(q)
	}()
	return q
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

	var q chan string

	if *async {
		q = filesAsync(base)
	} else {
		q = filesSync(base)
	}

	n := 0
	if *fsort {
		fs := []string{}
		for p := range q {
			if *progress {
				n++
				if n%10 == 0 {
					fmt.Fprintf(os.Stderr, "\r%d            \r", n)
				}
			}
			fs = append(fs, p)
		}
		sort.Strings(fs)
		for _, p := range fs {
			if *absolute {
				fmt.Println(p)
			} else {
				fmt.Println(p[len(base)+1:])
			}
		}
	} else {
		if *progress {
			for p := range q {
				n++
				if n%10 == 0 {
					fmt.Fprintf(os.Stderr, "\r%d            \r", n)
				}
				if *absolute {
					fmt.Println(p)
				} else {
					fmt.Println(p[len(base)+1:])
				}
			}
		} else {
			for p := range q {
				if *absolute {
					fmt.Println(p)
				} else {
					fmt.Println(p[len(base)+1:])
				}
			}
		}
	}
}
