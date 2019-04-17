package main

import (
	sha256 "crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/c2h5oh/datasize"
)

var imageExtentions = []string{"jpg", "jpeg", "gif", "png", "bmp", "webp"}

func hasImageExtention(filePath string) bool {
	for _, ext := range imageExtentions {
		if strings.HasSuffix(ext, filePath) {
			return true
		}
	}
	return false
}

// Image represents contents and properties of images
type Image struct {
	Path   string
	SHA256 [sha256.Size]byte
	Base64 []byte
}

// type lockedWriter struct {
// 	m      sync.Mutex
// 	Writer io.Writer
// }

// func (lw *lockedWriter) Write(b []byte) (n int, err error) {
// 	lw.m.Lock()
// 	defer lw.m.Unlock()
// 	return lw.Writer.Write(b)
// }

func main() {
	if len(os.Args) < 2 || 3 < len(os.Args) {
		fmt.Fprintf(os.Stderr, "usage: %s directory [maxFileSize]", os.Args[0])
		os.Exit(1)
	}

	location := os.Args[1]
	var limitSize datasize.ByteSize
	if len(os.Args) == 3 {
		err := limitSize.UnmarshalText([]byte(os.Args[2]))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse limit size. err: %s", err)
			os.Exit(1)
		}
	}
	chann := walk(location)
	for file := range chann {
		// if !hasImageExtention(file) {
		// 	continue
		// }
		out := make(chan []byte)
		go func(file string) {
			fmt.Fprintf(os.Stderr, "start marshalization: %s\n", file)
			defer close(out)
			img, err := marshalImage(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to read properties from images. err: %s", err)
			}
			j, err := json.Marshal(img)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to marshal JSON. err: %s", err)
			}
			out <- j
		}(file)
		fmt.Println(string(<-out))
	}
}

func marshalImage(filePath string) (*Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file. err: %s", err)
	}

	finfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info. err: %s", err)
	}
	fsize := finfo.Size()

	b := make([]byte, fsize)
	_, err = file.Read(b)
	if err != nil {
		return nil, fmt.Errorf("failed to read body of file. err: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	hash := make(chan [sha256.Size]byte)
	go func(b []byte) {
		defer close(hash)
		defer wg.Done()
		hash <- sha256.Sum256(b)
	}(b)

	wg.Add(1)
	b64 := make(chan []byte)
	go func(b []byte) {
		defer close(b64)
		defer wg.Done()
		res := make([]byte, base64.StdEncoding.EncodedLen(len(b)))
		base64.StdEncoding.Encode(res, b)
		b64 <- res
	}(b)

	img := &Image{Path: filePath}
	img.SHA256 = <-hash
	img.Base64 = <-b64

	wg.Wait()
	fmt.Fprintf(os.Stderr, "finish preceeding: %s\n", filePath)
	return img, nil
}

func walk(location string) chan string {
	chann := make(chan string)
	go func() {
		filepath.Walk(location, func(path string, _ os.FileInfo, _ error) (err error) {
			chann <- path
			return
		})
		defer close(chann)
	}()
	return chann
}
