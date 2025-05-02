package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type Data fmt.Stringer

type Directory struct {
	name string
	Data []Data
}

func newDirectory(name string, data []Data) Directory {
	return Directory{name: name, Data: data}
}

func (d Directory) String() string { return d.name }

type File struct {
	name string
	size int64
}

func newFile(dirEntry os.DirEntry) File {
	info, err := dirEntry.Info()
	if err != nil {
		return File{}
	}
	return File{name: dirEntry.Name(), size: info.Size()}
}

func (f File) String() string {
	if f.size == 0 {
		return f.name + " (empty)"
	}
	return fmt.Sprintf("%s (%db)", f.name, f.size)
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}

func dirTree(out io.Writer, path string, printFiles bool) error {
	data, err := collect(path, []Data{}, printFiles)
	if err != nil {
		return err
	}

	printTree(out, data, "")
	return nil
}

func collect(path string, dataList []Data, printFiles bool) ([]Data, error) {
	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return []Data{}, fmt.Errorf("failed to read directory %s due error: %w", path, err)
	}

	if len(dirEntries) > 1 {
		sort.Slice(dirEntries, func(i, j int) bool {
			return dirEntries[i].Name() < dirEntries[j].Name()
		})
	}

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			data, err := collect(filepath.Join(path, dirEntry.Name()), []Data{}, printFiles)
			if err != nil {
				return []Data{}, fmt.Errorf("failed to collect data due error: %w", err)
			}
			dataList = append(dataList, newDirectory(dirEntry.Name(), data))
		} else if printFiles {
			dataList = append(dataList, newFile(dirEntry))
		}
	}

	return dataList, nil
}

func printTree(out io.Writer, dataList []Data, startPostfix string) {
	if len(dataList) == 0 {
		return
	}

	innerPrint := func(data Data, prefix, postfix string) {
		fmt.Fprintf(out, "%s%s\n", prefix, data)
		if dir, ok := data.(Directory); ok {
			printTree(out, dir.Data, postfix)
		}
	}

	var prefix, postfix string
	for i, data := range dataList {
		fmt.Fprint(out, startPostfix)
		if i == len(dataList)-1 {
			prefix = "└───"
			postfix = startPostfix + "\t"
		} else {
			prefix = "├───"
			postfix = startPostfix + "│\t"
		}
		innerPrint(data, prefix, postfix)
	}
}
