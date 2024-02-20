package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	baseDirStats os.FileInfo
	baseDir      string
)

func main() {
	baseDir = os.Args[1]
	var err error
	baseDirStats, err = os.Stat(baseDir)
	if err != nil {
		panic("ERROR:" + err.Error())
	}
	if !baseDirStats.IsDir() {
		panic("ERROR: base is not a directory")
	}

	err = filepath.WalkDir(baseDir, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			fmt.Println("ERROR:", path, "// err:", err)
			return nil
		}
		fmt.Println(path)
		return nil
	})
	if err != nil {
		fmt.Println("ERROR:", err)
	}
}
