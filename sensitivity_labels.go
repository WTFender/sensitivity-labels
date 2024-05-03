package sensitivity_labels

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func Log(msgs []string, verbose bool) {
	if verbose {
		for _, m := range msgs {
			fmt.Println(m)
		}
	}
}

func ExitError(e error) {
	fmt.Println(e.Error())
	os.Exit(1)
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

func GetLabelInfoXml(filePath string) Labels {
	Log([]string{"open: " + filePath}, true)
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
	return labels
}

func CheckLabelInfoPath(dirPath string) (bool, string) {
	labelInfoPath := dirPath + "/docMetadata/labelInfo.xml"
	Log([]string{"checkLabelInfo " + labelInfoPath}, true)
	_, err := os.Stat(labelInfoPath)
	return (err == nil), labelInfoPath
}

func isExtensionFile(file os.FileInfo, exts []string) bool {
	for _, ext := range exts {
		if filepath.Ext(file.Name()) == ext {
			return true
		}
	}
	return false
}

func filterExtensionFiles(files []os.FileInfo, exts []string) []os.FileInfo {
	var filteredFiles []os.FileInfo
	for _, file := range files {
		if isExtensionFile(file, exts) {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return filteredFiles
}

// list files in a directory
func ListExtensionFiles(dir string, recursive bool, exts []string) []os.FileInfo {
	var files []fs.FileInfo

	if !recursive {
		items, err := os.ReadDir(dir)
		if err != nil {
			ExitError(err)
		}

		for _, item := range items {
			if !item.IsDir() {
				info, err := item.Info()
				if err != nil {
					ExitError(err)
				}
				files = append(files, info)
			}
		}

	} else {
		// recursively list files
		err := filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					ExitError(err)
				}
				if !info.IsDir() {
					files = append(files, info)
				}
				return nil
			})
		if err != nil {
			ExitError(err)
		}
	}
	return filterExtensionFiles(files, exts)
}
