package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		fmt.Println(f.Name)
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

var extensions = []string{".docx", ".xlsx", ".pptx"}

// flags
var tmpDir string
var verbose, dryrun, recurse bool

func init() {
	flag.BoolVar(&verbose, "verbose", false, "show diagnostic output")
	flag.BoolVar(&dryrun, "dry-run", false, "show results of set before applying")
	flag.BoolVar(&recurse, "recursive", false, "recurse through subdirectory files")
	flag.StringVar(&tmpDir, "tmp-dir", "./tmp", "temporary directory for file extraction")
}

func log(msgs []string) {
	if verbose {
		for _, m := range msgs {
			fmt.Println(m)
		}
	}
}

func exitError(e error) {
	fmt.Println(e.Error())
	os.Exit(1)
}

func printUsage(msg string) {
	usage := `%s
usage:
	labels.exe [--flags] get [path]
	labels.exe [--flags] set [path] [labelId] [tenantId]

commands	
	get: list sensitivity labels for the provided file or directory
	set: apply the provided sensitvity label ID to the provided file or directory

arguments
	path: path to the file or directory
	labelId: sensitivity label ID to apply
	tenantId: microsoft tenant ID to apply

flags
	--verbose: show diagnostic output
	--tmp-dir: temporary directory for file extraction
	--dry-run: show results of set command without applying
	--recurse: recurse through subdirectory files

examples
	labels.exe get "c:\path\to\directory"
	labels.exe get "c:\path\to\file.docx"
	labels.exe set "c:\path\to\file.docx" "1234-1234-1234" "4321-4321-4321"
	`
	fmt.Println(fmt.Sprintf(usage, msg))
}

/*
// helper function to filter a list of strings
func filter[T any](ss []T, test func(T) bool) (ret []T) {
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}
*/

func PrintResults(path string, files []fs.FileInfo) {
	for _, file := range files {
		fmt.Println(path + "/" + file.Name() + " " + strconv.FormatInt(file.Size(), 10) + " bytes")
	}
}

// Determine if the provided string is a file or directory path
func CheckPath(path string) fs.FileInfo {
	info, err := os.Stat(path)
	if err != nil {
		exitError(err)
	}
	return info
}

func isExtensionFile(file os.FileInfo) bool {
	for _, ext := range extensions {
		if filepath.Ext(file.Name()) == ext {
			return true
		}
	}
	return false
}

func filterExtensionFiles(files []os.FileInfo) []os.FileInfo {
	var filteredFiles []os.FileInfo
	for _, file := range files {
		if isExtensionFile(file) {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return filteredFiles
}

// list files in a directory
func ListFiles(dir string, recursive bool) []os.FileInfo {
	var files []fs.FileInfo

	if !recursive {
		items, err := os.ReadDir(dir)
		if err != nil {
			exitError(err)
		}

		for _, item := range items {
			if !item.IsDir() {
				info, err := item.Info()
				if err != nil {
					exitError(err)
				}
				files = append(files, info)
			}
		}

	} else {
		// recursively list files
		err := filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					exitError(err)
				}
				if !info.IsDir() {
					files = append(files, info)
				}
				return nil
			})
		if err != nil {
			exitError(err)
		}
	}
	return filterExtensionFiles(files)
}

func main() {
	flag.Parse()
	args := flag.Args()

	tmpDir = tmpDir + "/_/"

	log([]string{
		"args: " + strings.Join(os.Args, ", "),
		"parsed args: " + strings.Join(args, ", "),
	})

	if len(args) < 1 {
		printUsage("Error: missing command argument")
		os.Exit(1)
	} else if args[0] != "get" && args[0] != "set" {
		printUsage("Error: unsupported command " + args[0])
		os.Exit(1)
	} else if len(args) < 2 {
		printUsage("Error: missing path argument")
		os.Exit(1)
	}

	cmd := args[0]
	path := args[1]

	log([]string{
		"arg command: " + cmd,
		"arg path: " + path,
	})

	var files []fs.FileInfo

	pathInfo := CheckPath(path)
	if pathInfo.IsDir() {
		files = ListFiles(path, false)
	} else {
		// single file
		files = append(files, pathInfo)
	}

	if cmd == "get" {
		PrintResults(path, files)

	} else if cmd == "set" {
		if len(args) < 3 {
			printUsage("Error: missing labelId argument")
			os.Exit(1)
		} else if len(args) < 4 {
			printUsage("Error: missing tenantId argument")
			os.Exit(1)
		}

		labelId := args[2]
		tenantId := args[3]

		log([]string{
			"arg labelId: " + labelId,
			"arg tenantId: " + tenantId,
		})

		for _, file := range files {
			filePath := path + "/" + file.Name()
			tmpFileDir := tmpDir + file.Name()
			// clean start
			os.RemoveAll(tmpFileDir)
			log([]string{tmpFileDir})
			err := Unzip(filePath, tmpFileDir)

			// TODO modify labels here

			if err != nil {
				// clean up on error
				os.RemoveAll(tmpFileDir)
				exitError(err)
			}
			os.RemoveAll(tmpFileDir)
		}
	}
}
