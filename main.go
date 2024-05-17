package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-tty"

	"github.com/nyaosorg/go-readline-ny"

	"github.com/hymkor/csvi"
	"github.com/hymkor/csvi/uncsv"
)

func jisyoToTsv(r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadSlice('\n')
		if err != nil && err != io.EOF {
			return err
		}
		yomi, candidate, ok := bytes.Cut(line, []byte{' ', '/'})
		if ok {
			field := bytes.Split(candidate, []byte{'/'})
			field[len(field)-2] = append(field[len(field)-2], field[len(field)-1]...)
			field = field[:len(field)-1]
			w.Write(yomi)
			for _, f := range field {
				w.Write([]byte{'\t'})
				w.Write(f)
			}
		} else {
			w.Write(line)
		}
		if err == io.EOF {
			return nil
		}
	}
}

func splitLineBreak(line []byte) ([]byte, []byte) {
	L := len(line)
	if L >= 2 && line[L-2] == '\r' && line[L-1] == '\n' {
		return line[:L-2], line[L-2:]
	}
	if L >= 1 && line[L-1] == '\n' {
		return line[:L-1], line[L-1:]
	}
	return line, line[L:]
}

func saveAs(filename string, mode *uncsv.Mode, rows *csvi.Result) error {
	newFd, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer newFd.Close()
	w := bufio.NewWriter(newFd)
	defer w.Flush()
	rows.Each(func(row *uncsv.Row) bool {
		bin := row.Rebuild(mode)
		if len(row.Cell) <= 1 {
			w.Write(bin)
			return true
		}
		bin = bytes.Replace(bin, []byte{'\t'}, []byte{' ', '/'}, 1)
		bin = bytes.ReplaceAll(bin, []byte{'\t'}, []byte{'/'})

		text, eol := splitLineBreak(bin)
		w.Write(text)
		w.WriteByte('/')
		w.Write(eol)
		return true
	})
	return nil
}

func mains(args []string) error {
	if len(args) < 1 {
		return errors.New("too few arguments")
	}
	fd, err := os.Open(args[0])
	if err != nil {
		return err
	}
	pin, pout := io.Pipe()
	go func() {
		jisyoToTsv(fd, pout)
		pout.Close()
		fd.Close()
	}()

	cfg := &csvi.Config{
		Mode:      &uncsv.Mode{Comma: '\t'},
		CellWidth: 14,
		FixColumn: true,
	}
	cfg.SetEncoding("EUC-JP")

	row, err := cfg.Edit(pin, os.Stdout)
	pin.Close()
	if err != nil {
		return err
	}
	fmt.Printf("Save as %s ? [y/n] ", args[0])
	tty1, err := tty.Open()
	if err != nil {
		return err
	}
	defer tty1.Close()
	ans, err := readline.GetKey(tty1)
	if err != nil {
		return err
	}
	fmt.Println()
	if !strings.HasPrefix(strings.ToLower(ans), "y") {
		return nil
	}
	tmpFn := args[0] + ".tmp"
	if err := saveAs(tmpFn, cfg.Mode, row); err != nil {
		return err
	}
	os.Rename(args[0], args[0]+".bak")
	return os.Rename(tmpFn, args[0])
}

func main() {
	if err := mains(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
