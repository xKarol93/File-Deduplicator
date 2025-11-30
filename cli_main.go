//go:build cli

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/alexflint/go-arg"
)

func main() {
	var args struct {
		Directory string `arg:"-d, --directory,required" help:"Directory to scan for duplicate files"`
		Algorithm string `arg:"-a, --algorithm" default:"sha256" help:"Hashing algorithm (md5 | sha1 | sha256 | sha512 | blake3)"`
		Remove    bool   `arg:"-r, --delete" help:"Remove duplicates after scan (reads duplicate_files.txt)"`
		Json      bool   `arg:"-j, --json" help:"Also write duplicates.json with duplicate details"`
	}
	arg.MustParse(&args)
	fmt.Printf("Scanning directory: %s using algorithm: %s\n", args.Directory, args.Algorithm)

	out := make(chan string, 1024)
	dups := make(chan string, 1024)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for msg := range out {
			fmt.Print(msg)
		}
	}()
	go func() {
		defer wg.Done()
		for f := range dups {
			fmt.Println(f)
			if args.Remove {
				err := os.Remove(f)
				if err != nil {
					fmt.Printf("Error removing file %s: %v\n", f, err)
				} else {
					fmt.Printf("Removed duplicate file: %s\n", f)
				}
			}
		}
	}()

	ScanDirectory(args.Directory, args.Algorithm, args.Json, out, dups)
	close(dups)
	wg.Wait()

	if args.Remove {
		fmt.Println("Deletion flag set: removing duplicates listed in duplicate_files.txt...")
		f, err := os.Open("duplicate_files.txt")
		if err != nil {
			fmt.Println("Could not open duplicate_files.txt:", err)
			os.Exit(1)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		removed := 0
		skipped := 0
		for scanner.Scan() {
			path := strings.TrimSpace(scanner.Text())
			if path == "" {
				continue
			}
			if err := os.Remove(path); err != nil {
				fmt.Println("Skip:", path, "(", err, ")")
				skipped++
			} else {
				fmt.Println("Deleted:", path)
				removed++
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading duplicate_files.txt:", err)
		}
		fmt.Printf("Deletion complete: removed %d, skipped %d\n", removed, skipped)
	}
}
