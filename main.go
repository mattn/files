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

	"github.com/saracen/walker"
)

var (
	ignore        = flag.String("i", env(`FILES_IGNORE_PATTERN`, `^(\.git|\.hg|\.svn|_darcs|\.bzr)$`), "Ignore directory")
	ignoreenv     = flag.String("I", "", "Custom environment key for ignore")
	hidden        = flag.Bool("H", true, "Ignore hidden")
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

		var err error
		var fn func(path string, info os.FileInfo, err error) error
		if *directoryOnly {
			fn = func(path string, info os.FileInfo, err error) error {
				path = filepath.Clean(path)
				if path != "." {
					if *hidden && filepath.Base(path)[0] == '.' {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if info.IsDir() {
						if ignorere.MatchString(path) {
							return filepath.SkipDir
						}
						return processMatch(path, info)
					}
				}
				return nil
			}
		} else {
			fn = func(path string, info os.FileInfo, err error) error {
				path = filepath.Clean(path)
				if path != "." {
					if *hidden && filepath.Base(path)[0] == '.' {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if !info.IsDir() {
						if ignorere.MatchString(path) {
							return filepath.SkipDir
						}
						return processMatch(path, info)
					}
				}
				return nil
			}
		}
		err = filepath.Walk(base, fn)
		if err != nil && err != maxError {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		close(q)
	}()

	return q
}

func filesAsync(base string) chan string {
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

		var err error
		var fn func(path string, info os.FileInfo) error
		if *directoryOnly {
			fn = func(path string, info os.FileInfo) error {
				path = filepath.Clean(path)
				if path != "." {
					if *hidden && filepath.Base(path)[0] == '.' {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if info.IsDir() {
						if ignorere.MatchString(path) {
							return filepath.SkipDir
						}
						return processMatch(path, info)
					}
				}
				return nil
			}
		} else {
			fn = func(path string, info os.FileInfo) error {
				path = filepath.Clean(path)
				if path != "." {
					if *hidden && filepath.Base(path)[0] == '.' {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}
					if !info.IsDir() {
						if ignorere.MatchString(path) {
							return filepath.SkipDir
						}
						return processMatch(path, info)
					}
				}
				return nil
			}
		}
		cb := walker.WithErrorCallback(func(pathname string, err error) error {
			return nil
		})
		err = walker.Walk(base, fn, cb)

		if err != nil && err != maxError {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

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
	if *ignoreenv != "" {
		*ignore = os.Getenv(*ignoreenv)
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
				if _, err := os.Stdout.Write([]byte(filepath.Join(left, s) + "\n")); err != nil {
					os.Exit(2)
				}
			}
		} else {
			return func(s string) {
				if _, err := os.Stdout.Write([]byte(s + "\n")); err != nil {
					os.Exit(2)
				}
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
