package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"

	"github.com/saracen/walker"
)

const (
	name     = "files"
	version  = "0.3.3"
	revision = "HEAD"
)

type config struct {
	ignore        string
	ignoreenv     string
	hidden        bool
	absolute      bool
	fsort         bool
	match         string
	directoryOnly bool

	base        string
	left        string
	ignorere    *regexp.Regexp
	matchre     *regexp.Regexp
	showVersion bool
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

type walkFn func(path string, info os.FileInfo) error

func isHidden(cfg *config, path string) bool {
	return cfg.hidden && cfg.base != path && filepath.Base(path)[0] == '.'
}

func makeWalkFn(cfg *config, processMatch walkFn) walkFn {
	if cfg.directoryOnly {
		return func(path string, info os.FileInfo) error {
			path = filepath.Clean(path)
			if path == "." {
				return nil
			}
			if isHidden(cfg, path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if info.IsDir() {
				if cfg.ignorere.MatchString(path) {
					return filepath.SkipDir
				}
				return processMatch(path, info)
			}
			return nil
		}
	}
	return func(path string, info os.FileInfo) error {
		path = filepath.Clean(path)
		if path == "." {
			return nil
		}
		if isHidden(cfg, path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !info.IsDir() {
			if cfg.ignorere.MatchString(path) {
				return filepath.SkipDir
			}
			return processMatch(path, info)
		}
		return nil
	}
}

func makeMatchFn(cfg *config, q chan string) walkFn {
	if cfg.matchre != nil {
		return func(path string, info os.FileInfo) error {
			if !cfg.matchre.MatchString(info.Name()) {
				return nil
			}
			q <- filepath.ToSlash(path)
			return nil
		}
	}
	return func(path string, info os.FileInfo) error {
		q <- filepath.ToSlash(path)
		return nil
	}
}

func files(ctx context.Context, cfg *config) chan string {
	q := make(chan string, 20)

	base := cfg.base
	sep := string(os.PathSeparator)
	if !strings.HasSuffix(base, sep) {
		base += sep
	}

	cb := walker.WithErrorCallback(func(pathname string, err error) error {
		return nil
	})
	go func() {
		defer close(q)
		err := walker.WalkWithContext(ctx, base, makeWalkFn(cfg, makeMatchFn(cfg, q)), cb)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}()
	return q
}

func makePrintFn(cfg *config) func(string) {
	if cfg.absolute && !filepath.IsAbs(cfg.base) {
		return func(s string) {
			if _, err := os.Stdout.Write([]byte(filepath.Join(cfg.left, s) + "\n")); err != nil {
				os.Exit(2)
			}
		}
	}
	return func(s string) {
		if _, err := os.Stdout.Write([]byte(s + "\n")); err != nil {
			os.Exit(2)
		}
	}
}

func (cfg *config) doPrint(q chan string) {
	printFn := makePrintFn(cfg)
	if cfg.fsort {
		fs := []string{}
		for s := range q {
			fs = append(fs, s)
		}
		sort.Strings(fs)
		for _, s := range fs {
			printFn(s)
		}
	} else {
		for s := range q {
			printFn(s)
		}
	}
}

func run() int {
	var cfg config
	flag.StringVar(&cfg.ignore, "i", env(`FILES_IGNORE_PATTERN`, `^(\.git|\.hg|\.svn|_darcs|\.bzr)$`), "Ignore directory")
	flag.StringVar(&cfg.ignoreenv, "I", "", "Custom environment key for ignore")
	flag.BoolVar(&cfg.hidden, "H", true, "Ignore hidden")
	flag.BoolVar(&cfg.absolute, "a", false, "Display absolute path")
	flag.BoolVar(&cfg.fsort, "s", false, "Sort results")
	flag.StringVar(&cfg.match, "m", "", "Display matched files")
	flag.BoolVar(&cfg.directoryOnly, "d", false, "Directory only")
	flag.BoolVar(&cfg.showVersion, "v", false, "Show version")
	flag.Parse()

	if cfg.showVersion {
		fmt.Fprintf(os.Stdout, "%s\n", version)
		os.Exit(0)
	}

	var err error

	if cfg.match != "" {
		cfg.matchre, err = regexp.Compile(cfg.match)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if cfg.ignoreenv != "" {
		cfg.ignore = os.Getenv(cfg.ignoreenv)
	}
	cfg.ignorere, err = regexp.Compile(cfg.ignore)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	base := "."
	if flag.NArg() > 0 {
		base = flag.Arg(0)

		base = filepath.FromSlash(filepath.Clean(base))
		if runtime.GOOS == "windows" && base != "" && base[0] == '~' {
			base = filepath.Join(os.Getenv("USERPROFILE"), base[1:])
		}
	}

	left := base
	if cfg.absolute {
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
	cfg.base = base
	cfg.left = left

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		cancel()
		sc = nil
	}()

	cfg.doPrint(files(ctx, &cfg))

	if sc == nil {
		return 1
	}
	return 0
}

func main() {
	os.Exit(run())
}
