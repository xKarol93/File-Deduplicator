package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"sync"

	blake3 "github.com/zeebo/blake3"
)

type shaResult struct {
	FilePath string
	shaHash  string
}

type FileInfo struct {
	FilePath string
	FileSize int64
}

type JsonStruct struct {
	FileName string
	FilePath string
	FileSize int64
	Hash     string
}

func exploreDirectory(path string, out chan<- string) []FileInfo {
	file_infos := []FileInfo{}
	entries, _ := os.ReadDir(path)
	for _, entry := range entries {
		if entry.IsDir() {
			full := filepath.Join(path, entry.Name())
			children := exploreDirectory(full, out)
			file_infos = append(file_infos, children...)
			continue
		}
		file_info, err := os.Stat(path + string(os.PathSeparator) + entry.Name())
		if err != nil {
			continue
		}
		if file_info.Size() == 0 {
			continue
		}
		file_infos = append(file_infos, FileInfo{
			FilePath: path + string(os.PathSeparator) + entry.Name(),
			FileSize: file_info.Size(),
		})
	}
	out <- fmt.Sprintf("Explored %s, found %d files\n", path, len(file_infos))
	return file_infos
}

func get_hashsum(filePath string, algorithm string, wg *sync.WaitGroup, out chan<- shaResult) string {
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
	case "blake3":
		hasher = blake3.New()
	default:
		hasher = sha256.New()
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

func ScanDirectory(directory, algorithm string, writeJSON bool, out chan<- string, duplicates_chan chan<- string) {
	defer close(out)
	sorted_files := []FileInfo{}
	_, error := os.Stat(directory)
	if error != nil {
		out <- fmt.Sprintf("Directory does not exist: %s\n", directory)
		return
	}

	files := exploreDirectory(directory, out)
	sorted_files = append(sorted_files, files...)
	for i := 0; i < len(sorted_files)-1; i++ {
		for j := i + 1; j < len(sorted_files); j++ {
			if sorted_files[j].FileSize < sorted_files[i].FileSize {
				sorted_files[i], sorted_files[j] = sorted_files[j], sorted_files[i]
			}
		}
	}
	for _, file := range sorted_files {
		out <- fmt.Sprintf("Found file: %s (size: %d bytes)\n", file.FilePath, file.FileSize)
	}
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
	for _, f := range sorted_files {
		wg.Add(1)
		sem <- struct{}{}
		go func(p string) {
			defer func() { <-sem }()
			get_hashsum(p, algorithm, &wg, resultCh)
		}(f.FilePath)
	}

	// close resultCh when workers finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	hashMap := make(map[string]string)

	duplicates := 0

	var totalSaved int64
	json_contents := []JsonStruct{}
	_ = json_contents
	for r := range resultCh {
		if r.shaHash == "" {
			duplicates_chan <- fmt.Sprintf("ERR: %s -> hash failed\n", r.FilePath)
			continue
		}
		if _, ok := hashMap[r.shaHash]; ok {
			duplicates++
			if fi, err := os.Stat(r.FilePath); err == nil {
				size := fi.Size()
				totalSaved += size
				json_contents = append(json_contents, JsonStruct{
					FileName: filepath.Base(r.FilePath),
					FilePath: r.FilePath,
					FileSize: size,
					Hash:     r.shaHash,
				})
			}
			duplicated_files.WriteString(fmt.Sprintf("%s\n", r.FilePath))
			duplicates_chan <- r.FilePath
		} else {
			hashMap[r.shaHash] = r.FilePath
		}
	}
	if writeJSON {
		if json_data, err := json.MarshalIndent(json_contents, "", "  "); err == nil {
			if jf, err := os.Create("duplicates.json"); err == nil {
				_, _ = jf.Write(json_data)
				_ = jf.Close()
				out <- "Wrote duplicates.json\n"
			} else {
				out <- fmt.Sprintf("Failed to create duplicates.json: %v\n", err)
			}
		} else {
			out <- fmt.Sprintf("Failed to marshal duplicates to JSON: %v\n", err)
		}
	}
	duplicated_files.Close()

	out <- fmt.Sprintf("Total duplicate files: %d\n", duplicates)
	out <- fmt.Sprintf("Potential space to save: %dMB\n", totalSaved/1024/1024)
}

// CLI entrypoint is provided in cli_main.go with a build tag to avoid
// conflicting with GUI builds that also define a main.
