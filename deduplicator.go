package main

import (
	"flag"
	"os"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"crypto/md5"
	"sort"
)

func getMd5(file string) []byte {
	fmt.Println("getMd5", file)
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	bytes := make([]byte, 1024 * 10)
	h := md5.New()
	for {
		n, err := f.Read(bytes)
		if n == 0 {
			break
		}
		if err != nil {
			panic(err)
		}
		written, err := h.Write(bytes[:n])
		if err != nil {
			panic(err)
		}
		if n != written {
			panic(":(")
		}
	}
	md5value := h.Sum(nil)
	md5s[file] = md5value
	return md5value
}

var maxToWalk int

var walked int
// md5 => paths
var index map[string][]string
var md5s map[string][]byte

var sizeByPath map[string]int64

// md5 => total size of stuff with this md5
var totalSizesByMd5 map[string]int64

// dir => md5(md5s of sorted paths within)

type ByPath []string

func (a ByPath) Len() int           { return len(a) }
func (a ByPath) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByPath) Less(i, j int) bool { return a[i] < a[j] }

type ByTotalSize struct {
	md5s []string
	sizes []int64
}

func (a ByTotalSize) Len() int { return len(a.md5s) }
func (a ByTotalSize) Swap(i, j int) {
	a.md5s[i], a.md5s[j] = a.md5s[j], a.md5s[i]
	a.sizes[i], a.sizes[j] = a.sizes[j], a.sizes[i]
}
func (a ByTotalSize) Less(i, j int) bool {
	return a.sizes[i] < a.sizes[j]
}


func getDirMd5(dirname string) []byte {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}
	paths := make([]string, 0)
	for _, file := range files {
		path := dirname + "/" + file.Name()
		paths = append(paths, path)
	}
	sort.Sort(ByPath(paths))
	dirmd5 := md5.New()
	for _, path := range paths {
		dirmd5.Write(md5s[path])
	}
	return dirmd5.Sum(nil)
}

func getDirSize(dirname string) (size int64) {
	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		path := dirname + "/" + file.Name()
		if file.IsDir() {
			if subdirSize, ok := sizeByPath[path]; ok {
				size += subdirSize
			} else {
				size += getDirSize(path)
			}
		} else {
			size += file.Size()
		}
	}
	sizeByPath[dirname] = size
	return size
}

func walkDir(dirname string) {
	fmt.Println("walk", dirname, walked)
	if walked > maxToWalk {
		return
	}

	files, err := ioutil.ReadDir(dirname)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		if walked > maxToWalk {
			break
		}

		path := dirname + "/" + file.Name()
		if !file.IsDir() {
			md5 := getMd5(path)
			walked++
			md5str := hex.EncodeToString(md5)
			if _, ok := index[md5str]; !ok {
				index[md5str] = make([]string, 0)
			}
			index[md5str] = append(index[md5str], path)

			if _, ok := totalSizesByMd5[md5str]; !ok {
				totalSizesByMd5[md5str] = 0
			}
			totalSizesByMd5[md5str] += file.Size()

			// fmt.Println(path, md5)
		}
	}

	for _, file := range files {
		path := dirname + "/" + file.Name()
		if file.IsDir() {
			if file.Name() == ".dropbox.cache" {
				continue
			}
			walkDir(path)
		}
	}

	md5value := getDirMd5(dirname)
	md5s[dirname] = md5value
	md5str := hex.EncodeToString(md5value)
	if _, ok := index[md5str]; !ok {
		index[md5str] = make([]string, 0)
	}
	index[md5str] = append(index[md5str], dirname)

	// TODO: alsodo this by file?
	size := getDirSize(dirname)
	if _, ok := totalSizesByMd5[md5str]; !ok {
		totalSizesByMd5[md5str] = 0
	}
	totalSizesByMd5[md5str] += size
}

func main() {
	walked = 0
	index = make(map[string][]string)
	md5s = make(map[string][]byte)
	totalSizesByMd5 = make(map[string]int64)
	sizeByPath = make(map[string]int64)

	toWalk := flag.Int("max_to_walk", 10000, "maximum number of files to walk")
	flag.Parse()

	maxToWalk = *toWalk
	directories := flag.Args()

	for _, directory := range directories {
		walkDir(directory)
	}

	md5keys := make([]string, 0)
	md5sizes := make([]int64, 0)
	for md5key, size := range totalSizesByMd5 {
		md5keys = append(md5keys, md5key)
		md5sizes = append(md5sizes, size)
	}
	sort.Sort(ByTotalSize{md5s: md5keys, sizes: md5sizes})

	for i := 0; i < len(md5keys); i++ {
		md5 := md5keys[i]
		size := md5sizes[i]
		paths := index[md5]

		if len(paths) == 1 {
			continue
		}

		fmt.Println(md5, paths, size)
	}
}
