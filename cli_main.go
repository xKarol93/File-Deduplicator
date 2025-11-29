//go:build cli

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

func main() {
	removeDuplicates := false
	if len(os.Args) < 2 {
		fmt.Println("Usage: dedup <directory> [algorithm] [--delete]\nAlgorithms: md5 | sha1 | sha256 | sha512 (default: sha512)\nOptions: --delete  Remove duplicates after scan (reads duplicate_files.txt)")
		os.Exit(1)
	}
	directory := os.Args[1]
	algorithm := "sha512"
	if len(os.Args) >= 3 && os.Args[2] != "" {
		algorithm = os.Args[2]
	}
	doDelete := false
	if len(os.Args) >= 4 && os.Args[3] == "--delete" {
		doDelete = true
	}
	if len(os.Args) >= 4 && os.Args[3] == "true" {
		removeDuplicates = true
	}

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
			if removeDuplicates {
				err := os.Remove(f)
				if err != nil {
					fmt.Printf("Error removing file %s: %v\n", f, err)
				} else {
					fmt.Printf("Removed duplicate file: %s\n", f)
				}
			}
		}
	}()

	ScanDirectory(directory, algorithm, out, dups)
	close(dups)
	wg.Wait()

	if doDelete {
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
