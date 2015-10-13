package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

var ignore = flag.String("i", env(`FILES_IGNORE_PATTERN`, `^(\.git|\.hg|\.svn|_darcs|\.bzr)$`), "Ignore directory")
var progress = flag.Bool("p", false, "Progress message")
var async = flag.Bool("A", false, "Asynchronized find")
var absolute = flag.Bool("a", false, "Display absolute path")
var fsort = flag.Bool("s", false, "Sort results")
var match = flag.String("m", "", "Display matched files")
var maxfiles = flag.Int64("M", -1, "Max files")

var ignorere *regexp.Regexp
var matchre *regexp.Regexp
var maxcount = int64(^uint64(0) >> 1)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func filesSync(base string) chan string {
	q := make(chan string, 20)

	maxError := errors.New("Overflow max count")
	go func() {
		n := int64(0)
		sep := string(os.PathSeparator)
		if !strings.HasSuffix(base, sep) {
			base += sep
		}
		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if info == nil {
				return err
			}
			if !info.IsDir() {
				if ignorere.MatchString(info.Name()) {
					return nil
				}
				if matchre != nil && !matchre.MatchString(info.Name()) {
					return nil
				}

				n++
				if n > maxcount {
					return maxError
				}
				if *progress {
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

		if err != nil && err != maxError {
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
	n := int64(0)

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
				n++
				// This is pseudo handling because this is not atomic
				if n > maxcount {
					fmt.Fprintln(os.Stderr, "Overflow max count")
					return
				}
				if *progress {
					if n%10 == 0 {
						fmt.Fprintf(os.Stderr, "\r%d            \r", n)
					}
				}
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

	if *match != "" {
		matchre, err = regexp.Compile(*match)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
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

	sep := string(os.PathSeparator)
	if !strings.HasSuffix(base, sep) {
		base += sep
	}

	if *maxfiles > 0 {
		maxcount = *maxfiles
	}

	var q chan string

	if *async {
		q = filesAsync(base)
	} else {
		q = filesSync(base)
	}

	cwd, err := os.Getwd()
	if err == nil && strings.HasPrefix(base, cwd) {
		if !strings.HasSuffix(cwd, `\`) {
			cwd += `\`
		}
		base = cwd
	}

	printLine := func() func(string) {
		if *absolute {
			return func(s string) {
				fmt.Println(s)
			}
		} else {
			return func(s string) {
				fmt.Println(s[len(base):])
			}
		}
	}()
	if *fsort {
		fs := []string{}
		for p := range q {
			fs = append(fs, p)
		}
		sort.Strings(fs)
		for _, p := range fs {
			fmt.Println(p[len(base)+1:])
		}
	} else {
		for p := range q {
			printLine(p)
		}
	}
}
