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

var (
	ignore        = flag.String("i", env(`FILES_IGNORE_PATTERN`, `^(\.git|\.hg|\.svn|_darcs|\.bzr)$`), "Ignore directory")
	progress      = flag.Bool("p", false, "Progress message")
	async         = flag.Bool("A", false, "Asynchronized find")
	absolute      = flag.Bool("a", false, "Display absolute path")
	fsort         = flag.Bool("s", false, "Sort results")
	match         = flag.String("m", "", "Display matched files")
	maxfiles      = flag.Int64("M", -1, "Max files")
	directoryOnly = flag.Bool("d", false, "Directory only")
)

var (
	ignorere *regexp.Regexp
	matchre  *regexp.Regexp
	maxcount = int64(^uint64(0) >> 1)
	maxError = errors.New("Overflow max count")
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func filesSync(base string) chan string {
	q := make(chan string, 20)

	go func() {
		n := int64(0)
		sep := string(os.PathSeparator)
		if !strings.HasSuffix(base, sep) {
			base += sep
		}
		processMatch := func(path string, info os.FileInfo) error {
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

			return nil
		}

		err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if info == nil {
				return err
			}
			if *directoryOnly {
				if info.IsDir() {
					if ignorere.MatchString(info.Name()) {
						return filepath.SkipDir
					}
					return processMatch(path, info)
				}
			} else {
				if !info.IsDir() {
					if ignorere.MatchString(info.Name()) {
						return nil
					}
					return processMatch(path, info)
				} else {
					if ignorere.MatchString(info.Name()) {
						return filepath.SkipDir
					}
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

	var ferr error
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

		processMatch := func(p string, fi os.FileInfo) error {
			n++
			// This is pseudo handling because this is not atomic
			if n > maxcount {
				return maxError
			}
			if *progress {
				if n%10 == 0 {
					fmt.Fprintf(os.Stderr, "\r%d            \r", n)
				}
			}
			q <- filepath.ToSlash(filepath.Join(p, fi.Name()))

			return nil
		}

		for _, fi := range fis {
			if ignorere.MatchString(fi.Name()) {
				continue
			}
			if *directoryOnly {
				if fi.IsDir() {
					wg.Add(1)
					go fn(filepath.Join(p, fi.Name()))
					if ferr = processMatch(p, fi); ferr != nil {
						return
					}
				}
			} else {
				if fi.IsDir() {
					wg.Add(1)
					go fn(filepath.Join(p, fi.Name()))
				} else {
					if ferr = processMatch(p, fi); ferr != nil {
						return
					}
				}
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
		if ferr != nil {
			fmt.Fprintln(os.Stderr, ferr)
		}
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
		base = filepath.FromSlash(flag.Arg(0))
		if runtime.GOOS == "windows" && base != "" && base[0] == '~' {
			base = filepath.Join(os.Getenv("USERPROFILE"), base[1:])
		}
	}

	if *maxfiles > 0 {
		maxcount = *maxfiles
	}

	left := base
	if *absolute {
		if left, err = filepath.Abs(base); err != nil {
			left = filepath.Dir(left)
		}
	} else if !filepath.IsAbs(base) {
		if cwd, err := os.Getwd(); err == nil {
			if left, err = filepath.Rel(cwd, base); err == nil {
				base = left
			}
		}
	}

	var q chan string

	if *async {
		q = filesAsync(base)
	} else {
		q = filesSync(base)
	}

	printLine := func() func(string) {
		if *absolute && !filepath.IsAbs(base) {
			return func(s string) {
				fmt.Println(filepath.Join(left, s))
			}
		} else {
			return func(s string) {
				fmt.Println(s)
			}
		}
	}()
	if *fsort {
		fs := []string{}
		for s := range q {
			fs = append(fs, s)
		}
		sort.Strings(fs)
		for _, s := range fs {
			printLine(s)
		}
	} else {
		for s := range q {
			printLine(s)
		}
	}
}
