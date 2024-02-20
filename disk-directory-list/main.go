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
	baseDir = os.Getenv("LISTER_BASE_DIR")
	if baseDir == "" {
		baseDir = "/base"
	}

	var err error
	baseDirStats, err = os.Stat(baseDir)
	if err != nil {
		fmt.Println("ERROR:" + err.Error())
		return
	}
	if !baseDirStats.IsDir() {
		fmt.Println("ERROR: base is not a directory")
		return
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
