// chi.go (`ち`.go)
//
// Usage: chi [OPTIONS] [[FILE_OPTS]... FILE]...
//
// OPTIONS:
//   -i, --ignore-interrupts   ignore interrupt signals
//       --help                display this help and exit
//       --version             output version information and exit
//
// FILE_OPTS (apply to the next FILE only):
//   -a, --append              append to FILE (do not overwrite)
//   -b, --bare                write input as-is (keep ANSI escapes)
//   -c, --care                strip ANSI escapes (plain text)
//
// Copy standard input to each FILE, and also to standard output.
//
// Notes:
// - Output to stdout is always "as-is" (keeps ANSI escapes), so your terminal stays decorated.
// - For FILEs, default mode is "--bare" unless "--care" is specified for that FILE.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
)

const (
	appName    = "chi"
	appVersion = "0.1.0"
)

// ANSI CSI escape sequence matcher (fairly general).
// Examples: ESC [ 31m, ESC [ 0m, ESC [ 2K, ESC [ 1;32m, etc.
var ansiEscapeRegExpr = regexp.MustCompile(`\x1b\[[0-9;]*[ -/]*[@-~]`)

type fileMode int

const (
	// keep ANSI
	modeBare fileMode = iota
	
	// strip ANSI
	modeCare
	
	// I'm planning the `rare` for rhymy joking.
)

type pendingFileOpts struct {
	append bool
	mode   fileMode
}

type targetFile struct {
	path   string
	append bool
	mode   fileMode
}

func printHelp(whereToWrite *os.File) {
	fmt.Fprintf(whereToWrite, `Usage: %s [OPTIONS] [[FILE_OPTS]... FILE]...

OPTIONS:
  -i, --ignore-interrupts   ignore interrupt signals
      --help                display this help and exit
      --version             output version information and exit

FILE_OPTS (apply to the next FILE only):
  -a, --append              append to FILE (do not overwrite)
  -b, --bare                write input as-is (keep ANSI escapes)
  -c, --care                strip ANSI escapes (plain text)

Copy standard input to each FILE, and also to standard output.
`, appName)
}

func printVersion(whereToWrite *os.File) {
	fmt.Fprintf(whereToWrite, "%s %s\n", appName, appVersion)
}

// parseArgs parses argv (excluding argv[0]).
// Global options may appear anywhere (currently, there are no promises for the future; keep it leading for future compatibilities).
// FILE_OPTS apply only to the next FILE and are reset after consuming that FILE.
//
// Default per-file mode is "bare" (keep ANSI escapes) unless --care is specified.
func parseArgs(args []string) (ignoreInterrupts bool, targetFiles []targetFile, err error) {
	pend := pendingFileOpts{
		append: false,
		mode:   modeBare,
	}
	
	consumeFile := func(path string) {
		targetFiles = append(targetFiles, targetFile{
			path:   path,
			append: pend.append,
			mode:   pend.mode,
		})
		
		// Reset (FILE_OPTS apply to the next FILE only)
		pend = pendingFileOpts{
			append: false,
			mode:   modeBare,
		}
	}
	
	// Helper: expand short option clusters like -abc or -iab
	handleShortCluster := func(cluster string) error {
		// cluster does not include the leading "-"
		if cluster == "" {
			return fmt.Errorf("invalid option: '-'")
		}
		
		for _, ch := range cluster {
			switch ch {
			case 'i':
				ignoreInterrupts = true
			case 'a':
				pend.append = true
			case 'b':
				pend.mode = modeBare
			case 'c':
				pend.mode = modeCare
			default:
				return fmt.Errorf("unknown option: -%c", ch)
			}
		}
		
		return nil
	}
	
	// Walk tokens left-to-right
	for i := 0; i < len(args); i++ {
		token := args[i]
		
		if token == "--" {
			// Everything after "--" is treated as FILEs (no more option parsing).
			for j := i + 1; j < len(args); j++ {
				consumeFile(args[j])
			}
			
			return ignoreInterrupts, targetFiles, nil
		}
		
		if strings.HasPrefix(token, "--") {
			switch token {
			case "--help":
				printHelp(os.Stdout)
				os.Exit(0)
			case "--version":
				printVersion(os.Stdout)
				os.Exit(0)
			case "--ignore-interrupts":
				ignoreInterrupts = true
			case "--append":
				pend.append = true
			case "--bare":
				pend.mode = modeBare
			case "--care":
				pend.mode = modeCare
			default:
				return false, nil, fmt.Errorf("unknown option: %s", token)
			}
			continue
		}
		
		if strings.HasPrefix(token, "-") && (token != "-") {
			// short option or cluster
			if err := handleShortCluster(strings.TrimPrefix(token, "-")); err != nil {
				return false, nil, err
			}
			
			continue
		}
		
		// Not an option: treat as FILE
		consumeFile(token)
	}
	
	return ignoreInterrupts, targetFiles, nil
}

func openTarget(path string, appendMode bool) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY
	
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	
	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		return nil, err
	}
	
	return file, nil
}

func main() {
	ignoreInterrupts, targets, err := parseArgs(os.Args[1:])
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", appName)
		os.Exit(2)
	}
	
	if ignoreInterrupts {
		// Ignore SIGINT (Ctrl+C) and SIGTERM (common "interrupt-ish" signal).
		// If you want SIGTERM to still terminate, remove syscall.SIGTERM here.
		channels := make(chan os.Signal, 1)
		signal.Notify(channels, os.Interrupt, syscall.SIGTERM)
		
		go func() {
			for range channels {
				// Do nothing (ignoring)
			}
		}()
	}
	
	// Open all output files first
	type sink struct {
		mode   fileMode
		writer *bufio.Writer
		file   *os.File
	}
	sinks := make([]sink, 0, len(targets))
	
	for _, tgt := range targets {
		file, openErr := openTarget(tgt.path, tgt.append)
		if openErr != nil {
			fmt.Fprintf(os.Stderr, "%s: cannot open '%s': %v\n", appName, tgt.path, openErr)
			os.Exit(1)
		}
		
		sinks = append(sinks, sink{
			mode:   tgt.mode,
			writer: bufio.NewWriterSize(file, 64 * 1024),
			file:   file,
		})
	}
	
	// Ensure close
	defer func() {
		for _, sink := range sinks {
			_ = sink.writer.Flush()
			_ = sink.file.Close()
		}
	}()
	
	stdout := bufio.NewWriterSize(os.Stdout, 64 * 1024)
	defer stdout.Flush()
	
	stdin := bufio.NewReaderSize(os.Stdin, 64 * 1024)
	
	for {
		// keeps '\n' if present
		line, readErr := stdin.ReadBytes('\n')
		// assumes no case of (readErr != nil) && (len(line) > 0) here
		
		if readErr != nil {
			// EOF is normal termination; anything else is an error.
			if errors.Is(readErr, os.ErrClosed) {
				break
			}
			
			// bufio.Reader returns io.EOF at end; compare by string to avoid extra import.
			if readErr.Error() == "EOF" {
				break
			}
			
			fmt.Fprintf(os.Stderr, "%s: read error: %v\n", appName, readErr)
			os.Exit(1)
		}
		
		if len(line) <= 0 {
			continue
		}
		
		// Always write raw to stdout (keeping ANSI escapes)
		if _, writeErr := stdout.Write(line); writeErr != nil {
			fmt.Fprintf(os.Stderr, "%s: stdout write error: %v\n", appName, writeErr)
			os.Exit(1)
		}
		
		// Write to each file sink with per-file mode
		for _, sink := range sinks {
			var out []byte
			if sink.mode == modeBare {
				out = line
			} else {
				// care: strip ANSI escapes
				out = ansiEscapeRegExpr.ReplaceAll(line, nil)
			}
			
			if _, writeErr := sink.writer.Write(out); writeErr != nil {
				fmt.Fprintf(os.Stderr, "%s: file write error: %v\n", appName, writeErr)
				os.Exit(1)
			}
		}
	}
	
	// Flush sinks explicitly (defer also does it)
	if err := stdout.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: stdout flush error: %v\n", appName, err)
		os.Exit(1)
	}
	
	for _, sink := range sinks {
		if err := sink.writer.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "%s: file flush error: %v\n", appName, err)
			os.Exit(1)
		}
	}
	
	// A tiny sanity check to avoid "unused import" if you tweak later:
	_ = bytes.Compare
}
