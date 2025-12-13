# chi

`chi` is a small CLI utility similar to `tee`, but designed to handle
"slightly messy things" that `tee` does not.

The name **chi (ち)** comes from the hiragana character **ち**,
whose shape felt appropriate for a tool that *bends* standard streams
a little — not too much, but just enough.

## What it does

- Copies standard input to standard output (like `tee`)
- Writes the same input to one or more files
- Per-file options allow:
  - keeping ANSI escape sequences
  - stripping ANSI escape sequences
  - appending instead of overwriting

Typical use case:

```sh
tofu plan | chi --care plan.txt
```
Terminal output stays colored, while plan.txt is saved as plain text.

## Usage

```
Usage: chi [OPTIONS] [[FILE_OPTS]... FILE]...

OPTIONS:
  -i, --ignore-interrupts   ignore interrupt signals
      --help                display this help and exit
      --version             output version information and exit

FILE_OPTS (apply to the next FILE only):
  -a, --append              append to FILE (do not overwrite)
  -b, --bare                write input as-is (keep ANSI escapes)
  -c, --care                strip ANSI escapes (plain text)

Copy standard input to each FILE, and also to standard output.
```

## Build

From the project root:

```
go build ./cmd/chi
```

or explicitly name the output:

```
go build -o chi ./cmd/chi
```

or install it into your Go bin directory:

```
go install ./cmd/chi
```

After that, just use it as you see fit.

## License
- MIT License.

