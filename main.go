package main

import (
	"archive/zip"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type FileLabel struct {
	FilePath  string
	LabelInfo bool
	Labels    []Label
}

type Labels struct {
	XMLName xml.Name `xml:"labelList"`
	Labels  []Label  `xml:"label"`
}

type Label struct {
	XMLName     xml.Name `xml:"label"`
	Id          string   `xml:"id,attr"`
	SiteId      string   `xml:"siteId,attr"`
	Enabled     string   `xml:"enabled,attr"`
	Method      string   `xml:"method,attr"`
	ContentBits string   `xml:"contentBits,attr"`
	Removed     string   `xml:"removed,attr"`
}

var delimiter = " "
var extensions = []string{".docx", ".xlsx", ".pptx"}

// flags
var tmpDir string
var verbose, showLabeledOnly, showSummary, dryrun, recurse bool

func init() {
	flag.BoolVar(&verbose, "verbose", false, "show diagnostic output")
	flag.BoolVar(&showLabeledOnly, "labeled", false, "only show labeled files")
	flag.BoolVar(&showSummary, "summary", false, "display summary of results")
	flag.BoolVar(&dryrun, "dry-run", false, "show results of set before applying")
	flag.BoolVar(&recurse, "recursive", false, "recurse through subdirectory files")
	flag.StringVar(&tmpDir, "tmp-dir", "./tmp", "temporary directory for file extraction")
}

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
	set: apply the provided sensitivity label ID to the provided file or directory

arguments
	path: path to the file or directory
	labelId: sensitivity label ID to apply
	tenantId: microsoft tenant ID to apply

flags
	--labeled: only show files with labels
	--summary: show summary of results
	--recurse: recurse through subdirectory files
	--dry-run: show results of set command without applying
	--tmp-dir: temporary directory for file extraction
	--verbose: show diagnostic output

examples
	labels.exe --recursive --labeled get "c:\path\to\directory"
	labels.exe --summary set "c:\path\to\file.docx" "1234-1234-1234" "4321-4321-4321"`
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

func PrintFileLabelHeader() {
	fmt.Println(strings.Join([]string{
		"LabelInfo",
		"FilePath",
		"NumLabels",
		"Labels",
	}, delimiter))
}

func PrintFileLabel(fl FileLabel) {
	// true ./123.xlsx 1 [3de9faa6-9fe1-49b3-9a08-227a296b54a6 d5fe813e-0caa-432a-b2ac-d555aa91bd1c]
	labels := []string{}
	for _, label := range fl.Labels {
		labelStr := strings.ReplaceAll((label.Id + " " + label.SiteId), "{", "")
		labelStr = strings.ReplaceAll(labelStr, "}", "")
		labels = append(labels, labelStr)
	}
	// ./123.xlsx true [label1 label2]
	fmt.Println(strings.Join([]string{
		strconv.FormatBool(fl.LabelInfo),
		fl.FilePath,
		strconv.Itoa(len(fl.Labels)),           // Convert length to string
		"[" + strings.Join(labels, ", ") + "]", // Convert labels slice to a string
	}, delimiter))
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

func GetLabelInfoXml(filePath string) Labels {
	log([]string{"open: " + filePath})
	xmlFile, err := os.Open(filePath)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}
	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()
	// read our opened xmlFile as a byte array.
	byteValue, _ := io.ReadAll(xmlFile)
	// we initialize our Users array
	var labels Labels
	// we unmarshal our byteArray which contains our
	// xmlFiles content into 'users' which we defined above
	xml.Unmarshal(byteValue, &labels)
	// log([]string{"num labels: " + string(len(labels.Labels))})
	return labels
}

func CheckLabelInfoPath(dirPath string) (bool, string) {
	labelInfoPath := dirPath + "/docMetadata/labelInfo.xml"
	log([]string{"checkLabelInfo " + labelInfoPath})
	_, err := os.Stat(labelInfoPath)
	return (err == nil), labelInfoPath
}

func checkArgs(args []string) (string, string, string, string) {
	log([]string{
		"args: " + strings.Join(os.Args, ", "),
		"parsed args: " + strings.Join(args, ", "),
	})
	var cmd, path string
	labelId := ""
	tenantId := ""
	if len(args) < 1 {
		printUsage("Error: missing command argument")
		os.Exit(1)
	} else if len(args) < 2 {
		printUsage("Error: missing path argument")
		os.Exit(1)
	} else if args[0] != "get" && args[0] != "set" {
		printUsage("Error: unsupported command " + args[0])
		os.Exit(1)
	} else if args[0] == "set" && len(args) < 3 {
		printUsage("Error: missing labelId argument")
		os.Exit(1)
	} else if args[0] == "set" && len(args) < 4 {
		printUsage("Error: missing tenantId argument")
		os.Exit(1)
	} else if len(args) > 4 {
		printUsage("Error: too many arguments")
		os.Exit(1)
	}
	cmd = args[0]
	path = args[1]
	if len(args) == 4 {
		labelId = args[2]
		tenantId = args[3]
	}
	return cmd, path, labelId, tenantId
}

func main() {
	var files []fs.FileInfo
	var fileLabels []FileLabel

	// get command line arguments
	flag.Parse()
	args := flag.Args()
	cmd, path, labelId, tenantId := checkArgs(args)
	log([]string{
		"arg command: " + cmd,
		"arg path: " + path,
		"arg labelId: " + labelId,
		"arg tenantId: " + tenantId,
	})

	// check if path exists
	pathInfo, err := os.Stat(path)
	if err != nil {
		exitError(err)
	}

	// check if path is a directory, if so list files
	if pathInfo.IsDir() {
		files = ListFiles(path, false)
	} else {
		// single file
		files = append(files, pathInfo)
	}

	// print results header if files found
	if len(files) == 0 {
		fmt.Println("No files found")
		os.Exit(0)
	} else {
		PrintFileLabelHeader()
	}

	// iterate through files
	for _, file := range files {
		// create full path to file
		filePath := path + "/" + file.Name()
		// create temporary directory for file extraction
		tmpUnzipDir := tmpDir + "_" + file.Name()
		log([]string{"tmpUnzipDir: " + tmpUnzipDir})
		err := Unzip(filePath, tmpUnzipDir)
		if err != nil {
			// clean up on error
			exitError(err)
			os.RemoveAll(tmpUnzipDir)
		}
		// check extracted files for docMetadata/LabelInfo.xml
		labelInfoExists, labelInfoPath := CheckLabelInfoPath(tmpUnzipDir)
		fl := FileLabel{
			FilePath:  filePath,
			LabelInfo: labelInfoExists,
			Labels:    []Label{},
		}

		// if LabelInfo.xml exists, parse XML and return labels
		if fl.LabelInfo {
			labels := GetLabelInfoXml(labelInfoPath)
			fl.Labels = labels.Labels
		} else {
			log([]string{"LabelInfo.xml not found"})
		}

		// print results
		PrintFileLabel(fl)
		fileLabels = append(fileLabels, fl)

		// set new label
		if cmd == "set" {
			fmt.Println("TODO set labels")
			// TODO set labels here
		}

		os.RemoveAll(tmpUnzipDir)
	}

	// print results summary
	if showSummary {
		fmt.Println(fileLabels)
	}
}
