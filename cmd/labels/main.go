package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strconv"
	"strings"

	sl "github.com/WTFender/sensitivity_labels"
	flag "github.com/spf13/pflag"
)

// config.json can optionally be used
// to map label and tenant IDs to names
type LabelsConfig struct {
	Labels  map[string]string `json:"labels"`
	Tenants map[string]string `json:"tenants"`
}

var labelConfig = LabelsConfig{}

// flags
var extensionsCsv = ".docx,.xlsx,.pptx"
var tmpDir, config string
var verbose, showLabeledOnly, showSummary, dryrun, noCleanup, recurse bool
var delimiter = " " // TODO cleanup this

// logger
func log(msgs []string) {
	if verbose {
		for _, m := range msgs {
			fmt.Println(m)
		}
	}
}

func init() {
	flag.StringVar(&extensionsCsv, "extensions", extensionsCsv, "file extensions to search for (default: "+extensionsCsv+")")
	flag.BoolVar(&verbose, "verbose", false, "show diagnostic output")
	flag.BoolVar(&showLabeledOnly, "labeled", false, "only show labeled files")
	flag.BoolVar(&showSummary, "summary", false, "display summary of results")
	flag.StringVar(&config, "config", "", "path to JSON file containing ID to name mappings")
	flag.BoolVar(&dryrun, "dry-run", false, "show results of set before applying")
	flag.BoolVar(&recurse, "recursive", false, "recurse through subdirectory files")
	flag.StringVar(&tmpDir, "tmp-dir", "./", "temporary directory for file extraction")
	flag.BoolVar(&noCleanup, "no-cleanup", false, "do not remove temporary directory contents")
	flag.Usage = func() {
		printUsage("")
	}
}

func printUsage(msg string) {
	usage := `%s
usage:
	labels.exe [--flags] get <path>
	labels.exe [--flags] set <path> <labelId> <tenantId>

commands	
	get: list sensitivity labels for the provided file or directory
	set: apply the provided sensitivity label ID to the provided file or directory

arguments
	path: path to the file or directory
	labelId: sensitivity label ID to apply
	tenantId: microsoft tenant ID to apply

flags
%s
examples
	labels.exe --recursive --labeled get "c:\path\to\directory"
	labels.exe --summary set "c:\path\to\file.docx" "1234-1234-1234" "4321-4321-4321"`
	fmt.Println(fmt.Sprintf(usage, msg, flag.CommandLine.FlagUsages()))
}

func cleanup(path string) {
	log([]string{"cleanup: " + path})
	if !noCleanup {
		err := os.RemoveAll(path)
		if err != nil {
			log([]string{
				"cleanup error: " + path,
				err.Error(),
			})
		}
	}
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
	labelsArr := []string{}
	if showLabeledOnly && len(fl.Labels) == 0 {
		// --labeled flag, do not show files with no labels
		return
	}
	for _, label := range fl.Labels {
		labelStr := strings.ReplaceAll((label.Id + " " + label.SiteId), "{", "")
		labelStr = strings.ReplaceAll(labelStr, "}", "")
		labelsArr = append(labelsArr, labelStr)
	}
	combinedLabelStr := "[" + strings.Join(labelsArr, ", ") + "]"
	// resolve ids to names if config provided
	if config != "" {
		// for each key in labelConfig.Labels, replace id with name
		for labelId, labelName := range labelConfig.Labels {
			combinedLabelStr = strings.ReplaceAll(combinedLabelStr, labelId, labelName)
		}
		for tenantId, tenantName := range labelConfig.Tenants {
			combinedLabelStr = strings.ReplaceAll(combinedLabelStr, tenantId, tenantName)
		}
	}
	// ./123.xlsx true [label1 label2]
	fmt.Println(strings.Join([]string{
		strconv.FormatBool(fl.LabelInfo),
		fl.FilePath,
		strconv.Itoa(len(fl.Labels)), // Convert length to string
		combinedLabelStr,
	}, delimiter))
}

func checkArgs(args []string) (string, string, string, string, []string) {
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
	// check if extensions flag is set
	extensions := strings.Split(strings.TrimSpace(extensionsCsv), ",")
	if len(extensions) < 1 {
		fmt.Println("Skipping ID resolution, unable to parse JSON reference: " + config)
	}
	// check if config file is valid, ignore if not
	if config != "" {
		info, err := os.Stat(config)
		if err != nil || info.IsDir() {
			fmt.Println("Skipping ID resolution, unable to parse JSON reference: " + config)
			config = ""
		} else {
			labelConfig = parseLabelConfigJson(config)
			numIds := (len(labelConfig.Labels) + len(labelConfig.Tenants))
			log([]string{
				"loaded labelConfig: " + config,
				"labelConfig numEntries: " + strconv.Itoa(numIds),
			})
		}

	}
	if noCleanup {
		log([]string{"noCleanup: true"})
		fmt.Println("warn: temporary directory will not be removed")
	}
	if dryrun {
		log([]string{"dryrun: true"})
		fmt.Println("warn: dry-run enabled")
	}
	return cmd, path, labelId, tenantId, extensions
}

func main() {

	var files []fs.FileInfo
	var fileLabels []sl.FileLabel

	// get command line arguments
	flag.Parse()
	args := flag.Args()
	cmd, path, labelId, tenantId, extensions := checkArgs(args)

	log([]string{
		"arg command: " + cmd,
		"arg path: " + path,
		"arg labelId: " + labelId,
		"arg tenantId: " + tenantId,
		"arg extensions: " + strings.Join(extensions, ", "),
	})

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
		path = strings.ReplaceAll(path, pathInfo.Name(), "")
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
		log([]string{
			"filePath: " + filePath,
			"tmpUnzipDir: " + tmpUnzipDir,
		})
		unzipErr := sl.Unzip(filePath, tmpUnzipDir)
		if unzipErr != nil {
			// clean up on error
			sl.ExitError(unzipErr)
			cleanup(tmpUnzipDir)
		}
		// check extracted files for docMetadata/LabelInfo.xml
		labelInfoExists, labelInfoPath := sl.CheckLabelInfoPath(tmpUnzipDir)
		log([]string{
			"labelInfoExists: " + strconv.FormatBool(labelInfoExists),
			"checkLabelInfoPath: " + labelInfoPath,
		})
		fl := sl.FileLabel{
			FilePath:  filePath,
			LabelInfo: labelInfoExists,
			Labels:    []sl.Label{},
		}

		// if LabelInfo.xml exists, parse XML and return labels
		if fl.LabelInfo {
			log([]string{"open: " + filePath})
			labels := sl.GetLabelInfoXml(labelInfoPath)
			fl.Labels = labels.Labels
		} else {
			log([]string{"LabelInfo.xml not found"})
		}

		// set labels
		if cmd == "set" && unzipErr == nil {
			// set new label
			log([]string{"write: " + labelInfoPath})
			newLabels := sl.Labels{
				Labels: []sl.Label{
					{
						Id:          labelId,
						SiteId:      tenantId,
						Enabled:     "1",
						Method:      "Privileged",
						ContentBits: "0",
						Removed:     "0",
					},
				}}
			if dryrun {
				fl.Labels = newLabels.Labels
			} else {
				err := sl.SetLabels(tmpUnzipDir, filePath, labelInfoPath, newLabels)
				if err != nil {
					sl.ExitError(err)
				}
				fl.Labels = newLabels.Labels
			}
		}

		PrintFileLabel(fl)
		fileLabels = append(fileLabels, fl)
		cleanup(tmpUnzipDir)
	}

	// print results summary
	if showSummary {
		fmt.Println(fileLabels)
	}
}
