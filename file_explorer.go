package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type shaResult struct {
	FilePath string
	shaHash  string
}

func exploreDirectory(path string, out chan<- string) []string {
	file_list := []string{}
	entries, _ := os.ReadDir(path)
	for _, entry := range entries {
		if entry.IsDir() {
			full := filepath.Join(path, entry.Name())
			children := exploreDirectory(full, out)
			file_list = append(file_list, children...)
			continue
		}
		file_list = append(file_list, path+string(os.PathSeparator)+entry.Name())
	}
	out <- fmt.Sprintf("Explored %s, found %d files\n", path, len(file_list))
	return file_list
}

func get_sha512(filePath string, algorithm string, wg *sync.WaitGroup, out chan<- shaResult) string {
	defer wg.Done()
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	var hasher hash.Hash
	switch algorithm {
	case "md5":
		hasher = md5.New()
	case "sha1":
		hasher = sha1.New()
	case "sha256":
		hasher = sha256.New()
	case "sha512", "":
		hasher = sha512.New()
	default:
		hasher = sha512.New()
	}

	if _, err := io.Copy(hasher, file); err != nil {
		panic(err)
	}
	out <- shaResult{
		FilePath: filePath,
		shaHash:  hex.EncodeToString(hasher.Sum(nil)),
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func ScanDirectory(directory, algorithm string, out chan<- string, duplicates_chan chan<- string) {
	defer close(out)

	_, error := os.Stat(directory)
	if error != nil {
		out <- fmt.Sprintf("Directory does not exist: %s\n", directory)
		return
	}
	files := exploreDirectory(directory, out)
	out <- fmt.Sprintf("Found %d files\n", len(files))
	if len(files) == 0 {
		return
	}
	duplicated_files, err := os.Create("duplicate_files.txt")
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 128)
	resultCh := make(chan shaResult, len(files))

	// spawn hashers
	for _, f := range files {
		wg.Add(1)
		sem <- struct{}{}
		go func(p string) {
			defer func() { <-sem }()
			get_sha512(p, algorithm, &wg, resultCh)
		}(f)
	}

	// close resultCh when workers finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	hashMap := make(map[string]string)
	duplicates := 0
	var totalSaved int64
	for r := range resultCh {
		if r.shaHash == "" {
			duplicates_chan <- fmt.Sprintf("ERR: %s -> hash failed\n", r.FilePath)
			continue
		}
		if _, ok := hashMap[r.shaHash]; ok {
			duplicates++
			if fi, err := os.Stat(r.FilePath); err == nil {
				totalSaved += fi.Size()
			}
			duplicated_files.WriteString(fmt.Sprintf("%s\n", r.FilePath))
			duplicates_chan <- fmt.Sprintf(r.FilePath)
		} else {
			hashMap[r.shaHash] = r.FilePath
		}
	}
	duplicated_files.Close()

	out <- fmt.Sprintf("Total duplicate files: %d\n", duplicates)
	out <- fmt.Sprintf("Potential space to save: %dMB\n", totalSaved/1024/1024)
}

// CLI entrypoint is provided in cli_main.go with a build tag to avoid
// conflicting with GUI builds that also define a main.
