package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	sl "github.com/WTFender/sensitivity_labels"
)

type LabelsConfig struct {
	Labels  map[string]string `json:"labels"`
	Tenants map[string]string `json:"tenants"`
}

// flags
var tmpDir, resolve string
var verbose, showLabeledOnly, showSummary, dryrun, recurse bool
var delimiter = " "

var labelConfig = LabelsConfig{}
var extensions = []string{".docx", ".xlsx", ".pptx"}

func init() {
	flag.BoolVar(&verbose, "verbose", false, "show diagnostic output")
	flag.BoolVar(&showLabeledOnly, "labeled", false, "only show labeled files")
	flag.BoolVar(&showSummary, "summary", false, "display summary of results")
	flag.StringVar(&resolve, "resolve", "", "path to JSON file containing ID to name mappings")
	flag.BoolVar(&dryrun, "dry-run", false, "show results of set before applying")
	flag.BoolVar(&recurse, "recursive", false, "recurse through subdirectory files")
	flag.StringVar(&tmpDir, "tmp-dir", "./", "temporary directory for file extraction")
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

func parseLabelConfigJson(path string) LabelsConfig {
	var cfg LabelsConfig
	jsonFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	json.Unmarshal(byteValue, &cfg)
	return cfg
}

func PrintFileLabelHeader() {
	fmt.Println(strings.Join([]string{
		"LabelInfo",
		"FilePath",
		"NumLabels",
		"Labels",
	}, delimiter))
}

func PrintFileLabel(fl sl.FileLabel) {
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

func checkArgs(args []string) (string, string, string, string) {
	sl.Log([]string{
		"args: " + strings.Join(os.Args, ", "),
		"parsed args: " + strings.Join(args, ", "),
	}, verbose)
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
	// check if resolve JSON file is valid, ignore if not
	if resolve != "" {
		info, err := os.Stat(resolve)
		if err != nil || info.IsDir() {
			fmt.Println("Skipping ID resolution, unable to parse JSON reference: " + resolve)
			resolve = ""
		} else {
			labelConfig = parseLabelConfigJson(resolve)
			numIds := (len(labelConfig.Labels) + len(labelConfig.Tenants))
			sl.Log([]string{
				"loaded labelConfig: " + resolve,
				"labelConfig numEntries: " + strconv.Itoa(numIds),
			}, verbose)
		}

	}
	return cmd, path, labelId, tenantId
}

func main() {

	var files []fs.FileInfo
	var fileLabels []sl.FileLabel

	// get command line arguments
	flag.Parse()
	args := flag.Args()
	cmd, path, labelId, tenantId := checkArgs(args)

	sl.Log([]string{
		"arg command: " + cmd,
		"arg path: " + path,
		"arg labelId: " + labelId,
		"arg tenantId: " + tenantId,
	}, verbose)

	// check if path exists
	pathInfo, err := os.Stat(path)
	if err != nil {
		sl.ExitError(err)
	}

	// check if path is a directory, if so list files
	if pathInfo.IsDir() {
		files = sl.ListExtensionFiles(path, false, extensions)
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
		tmpUnzipDir := tmpDir + "/_" + file.Name()
		sl.Log([]string{"tmpUnzipDir: " + tmpUnzipDir}, verbose)
		err := sl.Unzip(filePath, tmpUnzipDir)
		if err != nil {
			// clean up on error
			sl.ExitError(err)
			os.RemoveAll(tmpUnzipDir)
		}
		// check extracted files for docMetadata/LabelInfo.xml
		labelInfoExists, labelInfoPath := sl.CheckLabelInfoPath(tmpUnzipDir)
		fl := sl.FileLabel{
			FilePath:  filePath,
			LabelInfo: labelInfoExists,
			Labels:    []sl.Label{},
		}

		// if LabelInfo.xml exists, parse XML and return labels
		if fl.LabelInfo {
			labels := sl.GetLabelInfoXml(labelInfoPath)
			fl.Labels = labels.Labels
		} else {
			sl.Log([]string{"LabelInfo.xml not found"}, verbose)
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
